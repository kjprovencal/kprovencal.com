package main

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"golang.org/x/crypto/bcrypt"
)

type galleryAPI struct {
	db     *badger.DB
	secret []byte
	cors   string
	store  objectStore
	unlock *slidingWindowLimiter
}

func mountGalleryRoutes(mux *http.ServeMux, db *badger.DB, secret []byte, cors string, store objectStore) {
	g := &galleryAPI{
		db:     db,
		secret: secret,
		cors:   cors,
		store:  store,
		unlock: newSlidingWindowLimiter(15*time.Minute, 8),
	}
	g.mount(mux)
}

func (g *galleryAPI) mount(mux *http.ServeMux) {
	if g.cors != "" {
		for _, p := range []string{
			"/admin/albums",
			"/admin/albums/{id}",
			"/admin/albums/{id}/uploads",
			"/admin/albums/{id}/photos/{photoId}",
			"/admin/albums/{id}/photos/{photoId}/complete",
		} {
			mux.HandleFunc("OPTIONS "+p, func(w http.ResponseWriter, r *http.Request) {
				adminPreflight(w, g.cors)
			})
		}
		for _, p := range []string{
			"/api/gallery/{token}",
			"/api/gallery/{token}/unlock",
			"/api/gallery/{token}/photos/{id}/download",
		} {
			mux.HandleFunc("OPTIONS "+p, func(w http.ResponseWriter, r *http.Request) {
				g.publicPreflight(w)
			})
		}
	}

	mux.Handle("GET /admin/albums", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.listAlbums))))
	mux.Handle("POST /admin/albums", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.createAlbum))))
	mux.Handle("GET /admin/albums/{id}", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.getAlbum))))
	mux.Handle("PATCH /admin/albums/{id}", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.patchAlbum))))
	mux.Handle("DELETE /admin/albums/{id}", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.deleteAlbum))))
	mux.Handle("POST /admin/albums/{id}/uploads", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.createUploads))))
	mux.Handle("POST /admin/albums/{id}/photos/{photoId}/complete", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.completePhoto))))
	mux.Handle("DELETE /admin/albums/{id}/photos/{photoId}", withAdminCORS(g.cors, requireAdmin(g.secret, http.HandlerFunc(g.deletePhoto))))

	mux.HandleFunc("GET /api/gallery/{token}", g.getPublicAlbum)
	mux.HandleFunc("POST /api/gallery/{token}/unlock", g.unlockAlbum)
	mux.HandleFunc("GET /api/gallery/{token}/photos/{id}/download", g.downloadPhoto)
}

func (g *galleryAPI) requireStore(w http.ResponseWriter, r *http.Request) bool {
	if g.store != nil {
		return true
	}
	slog.WarnContext(r.Context(), "gallery storage not configured", "path", r.URL.Path)
	writeJSON(w, http.StatusServiceUnavailable, errResp{"photo storage is not configured"})
	return false
}

type albumOut struct {
	ID         string    `json:"id"`
	Title      string    `json:"title"`
	SharePath  string    `json:"share_path"`
	CreatedAt  time.Time `json:"created_at"`
	PhotoCount int       `json:"photo_count"`
}

type photoOut struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Status   string `json:"status"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	WebURL   string `json:"web_url,omitempty"`
}

func albumToOut(album albumStored, photoCount int) albumOut {
	return albumOut{
		ID:         album.ID,
		Title:      album.Title,
		SharePath:  sharePath(album.Token),
		CreatedAt:  album.CreatedAt,
		PhotoCount: photoCount,
	}
}

func (g *galleryAPI) photoToOut(r *http.Request, photo photoStored, withWeb bool) photoOut {
	out := photoOut{
		ID:       photo.ID,
		Filename: photo.Filename,
		Status:   photo.Status,
		Width:    photo.Width,
		Height:   photo.Height,
	}
	if withWeb && photo.Status == photoStatusReady && photo.WebKey != "" && g.store != nil {
		url, err := g.store.PresignGet(r.Context(), photo.WebKey, webJPEGContentType, "", spacesViewTTL)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery presign view", "err", err, "photo_id", photo.ID)
		} else {
			out.WebURL = url
		}
	}
	return out
}

type albumCreateIn struct {
	Title string `json:"title"`
	Pin   string `json:"pin"`
}

func (g *galleryAPI) createAlbum(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	var in albumCreateIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}
	title, err := normalizeAlbumTitle(in.Title)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
		return
	}
	pin, err := normalizeAlbumPin(in.Pin)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pin), bcrypt.DefaultCost)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery hash pin", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not create album"})
		return
	}
	album, err := createAlbum(g.db, title, string(hash))
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery create album", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not create album"})
		return
	}
	writeJSON(w, http.StatusCreated, albumToOut(album, 0))
}

func (g *galleryAPI) listAlbums(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	albums, err := listAlbums(g.db)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery list albums", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not load albums"})
		return
	}
	out := make([]albumOut, 0, len(albums))
	for _, album := range albums {
		n, err := countPhotos(g.db, album.ID)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery count photos", "err", err, "album_id", album.ID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not load albums"})
			return
		}
		out = append(out, albumToOut(album, n))
	}
	writeJSON(w, http.StatusOK, out)
}

func (g *galleryAPI) albumFromPath(w http.ResponseWriter, r *http.Request) (albumStored, bool) {
	id := strings.TrimSpace(r.PathValue("id"))
	if !validObjectID(id) {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid album id"})
		return albumStored{}, false
	}
	album, err := getAlbum(g.db, id)
	if errors.Is(err, ErrAlbumNotFound) {
		writeJSON(w, http.StatusNotFound, errResp{"album not found"})
		return albumStored{}, false
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery get album", "err", err, "album_id", id)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not load album"})
		return albumStored{}, false
	}
	return album, true
}

type albumDetailOut struct {
	albumOut
	Photos []photoOut `json:"photos"`
}

func (g *galleryAPI) getAlbum(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	photos, err := listPhotos(g.db, album.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery list photos", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not load album"})
		return
	}
	detail := albumDetailOut{
		albumOut: albumToOut(album, len(photos)),
		Photos:   make([]photoOut, 0, len(photos)),
	}
	for _, photo := range photos {
		detail.Photos = append(detail.Photos, g.photoToOut(r, photo, true))
	}
	writeJSON(w, http.StatusOK, detail)
}

type albumPatchIn struct {
	Title string `json:"title"`
	Pin   string `json:"pin"`
}

func (g *galleryAPI) patchAlbum(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	var in albumPatchIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}
	title := strings.TrimSpace(in.Title)
	pin := strings.TrimSpace(in.Pin)
	if title == "" && pin == "" {
		writeJSON(w, http.StatusBadRequest, errResp{"title or pin is required"})
		return
	}
	var pinHash string
	setTitle := title != ""
	setPin := pin != ""
	if setTitle {
		normalized, err := normalizeAlbumTitle(title)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
			return
		}
		title = normalized
	}
	if setPin {
		normalized, err := normalizeAlbumPin(pin)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(normalized), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery hash pin", "err", err, "album_id", album.ID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not update album"})
			return
		}
		pinHash = string(hash)
	}
	updated, err := updateAlbum(g.db, album.ID, title, pinHash, setTitle, setPin)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery update album", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not update album"})
		return
	}
	n, err := countPhotos(g.db, album.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery count photos", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not update album"})
		return
	}
	writeJSON(w, http.StatusOK, albumToOut(updated, n))
}

func (g *galleryAPI) deleteAlbum(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	photos, err := listPhotos(g.db, album.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery list photos", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete album"})
		return
	}
	keys := make([]string, 0, len(photos)*2)
	for _, photo := range photos {
		keys = append(keys, photo.OriginalKey, photo.WebKey)
	}
	if err := g.store.Delete(r.Context(), keys...); err != nil {
		slog.ErrorContext(r.Context(), "gallery delete objects", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete album"})
		return
	}
	if err := deleteAlbumRecords(g.db, album, photos); err != nil {
		slog.ErrorContext(r.Context(), "gallery delete album", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete album"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type uploadFileIn struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Size    int64  `json:"size"`
	WebSize int64  `json:"web_size"`
}

type uploadBatchIn struct {
	Files []uploadFileIn `json:"files"`
}

type uploadSlotOut struct {
	PhotoID             string `json:"photo_id"`
	OriginalURL         string `json:"original_url"`
	OriginalContentType string `json:"original_content_type"`
	WebURL              string `json:"web_url"`
	WebContentType      string `json:"web_content_type"`
}

func (g *galleryAPI) createUploads(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	var in uploadBatchIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}
	if len(in.Files) == 0 {
		writeJSON(w, http.StatusBadRequest, errResp{"choose at least one photo"})
		return
	}
	if len(in.Files) > maxUploadBatch {
		writeJSON(w, http.StatusBadRequest, errResp{errUploadBatch.Error()})
		return
	}

	now := time.Now().UTC()
	photos := make([]photoStored, 0, len(in.Files))
	for i, file := range in.Files {
		name := cleanPhotoFilename(file.Name)
		if file.Size <= 0 {
			writeJSON(w, http.StatusBadRequest, errResp{name + ": " + errPhotoEmpty.Error()})
			return
		}
		if file.Size > maxOriginalBytes {
			writeJSON(w, http.StatusBadRequest, errResp{name + ": " + errPhotoTooLarge.Error()})
			return
		}
		ext, canonical, typeOK := classifyImage(file.Type, name)
		if !typeOK {
			writeJSON(w, http.StatusBadRequest, errResp{name + ": " + errPhotoType.Error()})
			return
		}
		if file.WebSize <= 0 {
			writeJSON(w, http.StatusBadRequest, errResp{name + ": " + errWebEmpty.Error()})
			return
		}
		if file.WebSize > maxWebBytes {
			writeJSON(w, http.StatusBadRequest, errResp{name + ": " + errWebTooLarge.Error()})
			return
		}
		id, err := randomID(8)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery photo id", "err", err, "album_id", album.ID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare upload"})
			return
		}
		photos = append(photos, photoStored{
			ID:          id,
			AlbumID:     album.ID,
			Filename:    name,
			ContentType: canonical,
			OriginalKey: originalObjectKey(album.ID, id, ext),
			WebKey:      webObjectKey(album.ID, id),
			Status:      photoStatusPending,
			CreatedAt:   now.Add(time.Duration(i)),
		})
	}

	if err := insertPhotos(g.db, photos); err != nil {
		slog.ErrorContext(r.Context(), "gallery insert photos", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare upload"})
		return
	}

	out := make([]uploadSlotOut, 0, len(photos))
	for i, photo := range photos {
		file := in.Files[i]
		originalURL, err := g.store.PresignPut(r.Context(), photo.OriginalKey, photo.ContentType, file.Size, spacesPresignTTL)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery presign original", "err", err, "photo_id", photo.ID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare upload"})
			return
		}
		webURL, err := g.store.PresignPut(r.Context(), photo.WebKey, webJPEGContentType, file.WebSize, spacesPresignTTL)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery presign web", "err", err, "photo_id", photo.ID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare upload"})
			return
		}
		out = append(out, uploadSlotOut{
			PhotoID:             photo.ID,
			OriginalURL:         originalURL,
			OriginalContentType: photo.ContentType,
			WebURL:              webURL,
			WebContentType:      webJPEGContentType,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"uploads": out})
}

type completePhotoIn struct {
	Width  int `json:"width"`
	Height int `json:"height"`
}

func (g *galleryAPI) completePhoto(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	photoID := strings.TrimSpace(r.PathValue("photoId"))
	if !validObjectID(photoID) {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid photo id"})
		return
	}
	var in completePhotoIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}
	if in.Width < 0 || in.Height < 0 || in.Width > maxPhotoDimension || in.Height > maxPhotoDimension {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid image size"})
		return
	}
	photo, err := getPhoto(g.db, album.ID, photoID)
	if errors.Is(err, ErrPhotoNotFound) {
		writeJSON(w, http.StatusNotFound, errResp{"photo not found"})
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery get photo", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not finish upload"})
		return
	}
	for _, key := range []string{photo.OriginalKey, photo.WebKey} {
		exists, err := g.store.Head(r.Context(), key)
		if err != nil {
			slog.ErrorContext(r.Context(), "gallery head object", "err", err, "photo_id", photoID)
			writeJSON(w, http.StatusInternalServerError, errResp{"could not finish upload"})
			return
		}
		if !exists {
			writeJSON(w, http.StatusBadRequest, errResp{"upload incomplete"})
			return
		}
	}
	updated, err := markPhotoReady(g.db, album.ID, photoID, in.Width, in.Height)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery mark ready", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not finish upload"})
		return
	}
	writeJSON(w, http.StatusOK, g.photoToOut(r, updated, true))
}

func (g *galleryAPI) deletePhoto(w http.ResponseWriter, r *http.Request) {
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromPath(w, r)
	if !ok {
		return
	}
	photoID := strings.TrimSpace(r.PathValue("photoId"))
	if !validObjectID(photoID) {
		writeJSON(w, http.StatusBadRequest, errResp{"invalid photo id"})
		return
	}
	photo, err := getPhoto(g.db, album.ID, photoID)
	if errors.Is(err, ErrPhotoNotFound) {
		writeJSON(w, http.StatusNotFound, errResp{"photo not found"})
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery get photo", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete photo"})
		return
	}
	if err := g.store.Delete(r.Context(), photo.OriginalKey, photo.WebKey); err != nil {
		slog.ErrorContext(r.Context(), "gallery delete photo objects", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete photo"})
		return
	}
	if err := deletePhotoRecord(g.db, album.ID, photoID); err != nil {
		slog.ErrorContext(r.Context(), "gallery delete photo", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not delete photo"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
