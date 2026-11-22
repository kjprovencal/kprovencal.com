package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const galleryCookieName = "gallery_session"

const gallerySessionTTL = 14 * 24 * time.Hour

type gallerySessionPayload struct {
	AlbumID string `json:"album_id"`
	Exp     int64  `json:"exp"`
}

func signGallerySession(secret []byte, albumID string, exp time.Time) (string, error) {
	body, err := json.Marshal(gallerySessionPayload{AlbumID: albumID, Exp: exp.Unix()})
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parseGallerySession(secret []byte, raw string) (string, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return "", errors.New("invalid session")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return "", errors.New("bad signature")
	}
	var p gallerySessionPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", err
	}
	if p.AlbumID == "" || time.Now().Unix() > p.Exp {
		return "", errors.New("expired")
	}
	return p.AlbumID, nil
}

func (g *galleryAPI) setPublicCORS(w http.ResponseWriter) {
	if g.cors == "" {
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", g.cors)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}

func (g *galleryAPI) publicPreflight(w http.ResponseWriter) {
	if g.cors == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	g.setPublicCORS(w)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.WriteHeader(http.StatusNoContent)
}

func (g *galleryAPI) setGalleryCookie(w http.ResponseWriter, albumID string) error {
	exp := time.Now().Add(gallerySessionTTL)
	val, err := signGallerySession(g.secret, albumID, exp)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     galleryCookieName,
		Value:    val,
		Path:     "/",
		MaxAge:   int(time.Until(exp).Seconds()),
		HttpOnly: true,
		SameSite: cookieSameSite(),
		Secure:   isProd(),
	})
	return nil
}

func (g *galleryAPI) guestAlbumID(r *http.Request) (string, bool) {
	c, err := r.Cookie(galleryCookieName)
	if err != nil {
		return "", false
	}
	id, err := parseGallerySession(g.secret, c.Value)
	if err != nil {
		return "", false
	}
	return id, true
}

func (g *galleryAPI) albumFromToken(w http.ResponseWriter, r *http.Request) (albumStored, bool) {
	token := strings.ToLower(strings.TrimSpace(r.PathValue("token")))
	if !validObjectID(token) {
		writeJSON(w, http.StatusNotFound, errResp{"album not found"})
		return albumStored{}, false
	}
	album, err := getAlbumByToken(g.db, token)
	if errors.Is(err, ErrAlbumNotFound) {
		writeJSON(w, http.StatusNotFound, errResp{"album not found"})
		return albumStored{}, false
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery lookup", "err", err)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not load album"})
		return albumStored{}, false
	}
	return album, true
}

type publicAlbumOut struct {
	Title  string     `json:"title"`
	Locked bool       `json:"locked"`
	Photos []photoOut `json:"photos,omitempty"`
}

func (g *galleryAPI) getPublicAlbum(w http.ResponseWriter, r *http.Request) {
	g.setPublicCORS(w)
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromToken(w, r)
	if !ok {
		return
	}
	guestID, unlocked := g.guestAlbumID(r)
	if !unlocked || guestID != album.ID {
		writeJSON(w, http.StatusOK, publicAlbumOut{Title: album.Title, Locked: true})
		return
	}
	photos, err := listPhotos(g.db, album.ID)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery list photos", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not load album"})
		return
	}
	out := publicAlbumOut{
		Title:  album.Title,
		Locked: false,
		Photos: make([]photoOut, 0, len(photos)),
	}
	for _, photo := range photos {
		if photo.Status != photoStatusReady {
			continue
		}
		out.Photos = append(out.Photos, g.photoToOut(r, photo, true))
	}
	writeJSON(w, http.StatusOK, out)
}

type unlockIn struct {
	Pin string `json:"pin"`
}

// rateLimitClientIP is the address counted by the gallery PIN limiter.
// nginx appends the TCP client with $proxy_add_x_forwarded_for, so the last hop is the one it saw.
// The first hop is whatever the client sent. Without TRUST_PROXY, only the direct connection is used.
func rateLimitClientIP(r *http.Request) string {
	if trustProxy() {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		for i := len(parts) - 1; i >= 0; i-- {
			hop := strings.TrimSpace(parts[i])
			if hop != "" {
				return hop
			}
		}
	}
	return clientIP(r)
}

func (g *galleryAPI) unlockAlbum(w http.ResponseWriter, r *http.Request) {
	g.setPublicCORS(w)
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromToken(w, r)
	if !ok {
		return
	}
	key := rateLimitClientIP(r) + "|" + album.Token
	if g.unlock.blocked(key) {
		writeJSON(w, http.StatusTooManyRequests, errResp{"too many attempts"})
		return
	}
	var in unlockIn
	if !decodeJSONBody(w, r, &in, false) {
		return
	}
	pin, err := normalizeAlbumPin(in.Pin)
	if err != nil {
		g.unlock.allow(key)
		writeJSON(w, http.StatusUnauthorized, errResp{"incorrect pin"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(album.PinHash), []byte(pin)) != nil {
		g.unlock.allow(key)
		slog.InfoContext(r.Context(), "gallery unlock failed", "album_id", album.ID, "remote_addr", requestRemoteAddr(r))
		writeJSON(w, http.StatusUnauthorized, errResp{"incorrect pin"})
		return
	}
	if err := g.setGalleryCookie(w, album.ID); err != nil {
		slog.ErrorContext(r.Context(), "gallery session cookie", "err", err, "album_id", album.ID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not unlock album"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (g *galleryAPI) downloadPhoto(w http.ResponseWriter, r *http.Request) {
	g.setPublicCORS(w)
	if !g.requireStore(w, r) {
		return
	}
	album, ok := g.albumFromToken(w, r)
	if !ok {
		return
	}
	guestID, unlocked := g.guestAlbumID(r)
	if !unlocked || guestID != album.ID {
		writeJSON(w, http.StatusUnauthorized, errResp{"incorrect pin"})
		return
	}
	variant, err := parsePhotoVariant(r.URL.Query().Get("variant"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
		return
	}
	photoID := strings.TrimSpace(r.PathValue("id"))
	if !validObjectID(photoID) {
		writeJSON(w, http.StatusNotFound, errResp{"photo not found"})
		return
	}
	photo, err := getPhoto(g.db, album.ID, photoID)
	if errors.Is(err, ErrPhotoNotFound) || (err == nil && photo.Status != photoStatusReady) {
		writeJSON(w, http.StatusNotFound, errResp{"photo not found"})
		return
	}
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery get photo", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare download"})
		return
	}
	objectKey, err := photoObjectKey(photo, variant)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errResp{err.Error()})
		return
	}
	filename := downloadFilename(photo, variant)
	url, err := g.store.PresignGet(r.Context(), objectKey, downloadContentType(photo, variant), filename, spacesPresignTTL)
	if err != nil {
		slog.ErrorContext(r.Context(), "gallery presign download", "err", err, "photo_id", photoID)
		writeJSON(w, http.StatusInternalServerError, errResp{"could not prepare download"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"url":      url,
		"filename": filename,
	})
}
