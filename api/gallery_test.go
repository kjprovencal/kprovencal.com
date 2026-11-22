package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

type memStore struct {
	mu      sync.Mutex
	objects map[string]bool
	puts    []string
	putLens []int64
	gets    []string
	getTTL  []time.Duration
}

func newMemStore() *memStore {
	return &memStore{objects: map[string]bool{}}
}

func (m *memStore) PresignPut(_ context.Context, key, contentType string, contentLength int64, _ time.Duration) (string, error) {
	m.mu.Lock()
	m.puts = append(m.puts, key+" "+contentType)
	m.putLens = append(m.putLens, contentLength)
	m.mu.Unlock()
	return "https://files.test/put/" + url.PathEscape(key), nil
}

func (m *memStore) PresignGet(_ context.Context, key, contentType, downloadName string, expires time.Duration) (string, error) {
	q := url.Values{}
	q.Set("type", contentType)
	q.Set("name", downloadName)
	raw := "https://files.test/get/" + url.PathEscape(key) + "?" + q.Encode()
	m.mu.Lock()
	m.gets = append(m.gets, raw)
	m.getTTL = append(m.getTTL, expires)
	m.mu.Unlock()
	return raw, nil
}

func (m *memStore) lastPutLens() []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]int64, len(m.putLens))
	copy(out, m.putLens)
	return out
}

func (m *memStore) lastGetTTL() time.Duration {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.getTTL) == 0 {
		return 0
	}
	return m.getTTL[len(m.getTTL)-1]
}

func (m *memStore) Head(_ context.Context, key string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.objects[key], nil
}

func (m *memStore) Delete(_ context.Context, keys ...string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		delete(m.objects, key)
	}
	return nil
}

func (m *memStore) add(keys ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, key := range keys {
		m.objects[key] = true
	}
}

func (m *memStore) has(key string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.objects[key]
}

func newGalleryMux(t *testing.T, store objectStore, unlockMax int) (*galleryAPI, *http.ServeMux, []byte) {
	t.Helper()
	if unlockMax < 1 {
		unlockMax = 8
	}
	secret := bytes.Repeat([]byte("s"), 32)
	g := &galleryAPI{
		db:     newTestDB(t),
		secret: secret,
		store:  store,
		unlock: newSlidingWindowLimiter(15*time.Minute, unlockMax),
	}
	mux := http.NewServeMux()
	g.mount(mux)
	return g, mux, secret
}

func adminReq(t *testing.T, method, target, body string, secret []byte) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	val, err := signSession(secret, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: val})
	return req
}

func doReq(mux *http.ServeMux, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dest any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dest); err != nil {
		t.Fatalf("json: %v body %s", err, rec.Body.String())
	}
}

func TestClassifyImage(t *testing.T) {
	t.Parallel()
	ext, ct, ok := classifyImage("image/jpg", "x.bin")
	if !ok || ext != ".jpg" || ct != "image/jpeg" {
		t.Fatalf("jpeg got %s %s %v", ext, ct, ok)
	}
	ext, ct, ok = classifyImage("", "pic.HEIC")
	if !ok || ext != ".heic" || ct != "image/heic" {
		t.Fatalf("heic got %s %s %v", ext, ct, ok)
	}
	if _, _, ok := classifyImage("text/plain", "notes.txt"); ok {
		t.Fatal("expected reject")
	}
}

func TestPhotoVariantKeys(t *testing.T) {
	t.Parallel()
	photo := photoStored{OriginalKey: "albums/a/p/original.jpg", WebKey: "albums/a/p/web.jpg", Filename: "Day.HEIC", ContentType: "image/heic"}
	web, err := photoObjectKey(photo, variantWeb)
	if err != nil || web != photo.WebKey {
		t.Fatalf("web key %s %v", web, err)
	}
	orig, err := photoObjectKey(photo, variantOriginal)
	if err != nil || orig != photo.OriginalKey {
		t.Fatalf("original key %s %v", orig, err)
	}
	if downloadFilename(photo, variantWeb) != "Day.jpg" {
		t.Fatalf("web name %s", downloadFilename(photo, variantWeb))
	}
	if downloadFilename(photo, variantOriginal) != "Day.HEIC" {
		t.Fatalf("original name %s", downloadFilename(photo, variantOriginal))
	}
	if _, err := parsePhotoVariant("print"); err == nil {
		t.Fatal("expected invalid variant")
	}
}

func TestOpenSpacesFromEnvUnset(t *testing.T) {
	t.Setenv("SPACES_KEY", "")
	t.Setenv("SPACES_SECRET", "")
	t.Setenv("SPACES_BUCKET", "")
	t.Setenv("SPACES_REGION", "")
	t.Setenv("SPACES_ENDPOINT", "")
	store, err := openSpacesFromEnv(context.Background())
	if err != nil || store != nil {
		t.Fatalf("store %v err %v", store, err)
	}

	t.Setenv("SPACES_KEY", "key")
	_, err = openSpacesFromEnv(context.Background())
	if err == nil {
		t.Fatal("expected partial config error")
	}
}

func TestGalleryRequiresStorage(t *testing.T) {
	t.Parallel()
	_, mux, secret := newGalleryMux(t, nil, 8)
	rec := doReq(mux, adminReq(t, http.MethodPost, "/admin/albums", `{"title":"Day","pin":"1234"}`, secret))
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "not configured") {
		t.Fatalf("admin %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(mux, httptest.NewRequest(http.MethodGet, "/api/gallery/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("public %d %s", rec.Code, rec.Body.String())
	}
}

func TestGalleryAlbumGuestFlow(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	g, mux, secret := newGalleryMux(t, store, 8)

	rec := doReq(mux, adminReq(t, http.MethodPost, "/admin/albums", `{"title":"Rehearsal","pin":"2468"}`, secret))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created albumOut
	decodeBody(t, rec, &created)
	if created.SharePath == "" || strings.Contains(rec.Body.String(), "pin_hash") {
		t.Fatalf("create body %s", rec.Body.String())
	}
	token := strings.TrimPrefix(created.SharePath, "/gallery/")
	if len(token) != 32 {
		t.Fatalf("token %q", token)
	}

	rec = doReq(mux, adminReq(t, http.MethodPost, "/admin/albums/"+created.ID+"/uploads", `{
		"files":[
			{"name":"../Day One.JPG","type":"image/jpeg","size":1200,"web_size":400},
			{"name":"notes.txt","type":"text/plain","size":12,"web_size":10}
		]
	}`, secret))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad type %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, adminReq(t, http.MethodPost, "/admin/albums/"+created.ID+"/uploads", `{
		"files":[{"name":"huge.jpg","type":"image/jpeg","size":1200,"web_size":13000000}]
	}`, secret))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "12 MB") {
		t.Fatalf("web size %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, adminReq(t, http.MethodPost, "/admin/albums/"+created.ID+"/uploads", `{
		"files":[{"name":"Day One.JPG","type":"image/jpeg","size":1200,"web_size":400}]
	}`, secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("upload %d %s", rec.Code, rec.Body.String())
	}
	var prepared struct {
		Uploads []uploadSlotOut `json:"uploads"`
	}
	decodeBody(t, rec, &prepared)
	if len(prepared.Uploads) != 1 {
		t.Fatalf("uploads %#v", prepared.Uploads)
	}
	slot := prepared.Uploads[0]
	photo, err := getPhoto(g.db, created.ID, slot.PhotoID)
	if err != nil {
		t.Fatal(err)
	}
	wantOriginal := "albums/" + created.ID + "/" + slot.PhotoID + "/original.jpg"
	wantWeb := "albums/" + created.ID + "/" + slot.PhotoID + "/web.jpg"
	if photo.OriginalKey != wantOriginal || photo.WebKey != wantWeb || photo.Filename != "Day One.JPG" {
		t.Fatalf("keys %#v", photo)
	}
	if slot.OriginalContentType != "image/jpeg" || slot.WebContentType != "image/jpeg" {
		t.Fatalf("types %#v", slot)
	}
	if !strings.Contains(slot.OriginalURL, url.PathEscape(wantOriginal)) {
		t.Fatalf("original url %s", slot.OriginalURL)
	}
	if lens := store.lastPutLens(); len(lens) != 2 || lens[0] != 1200 || lens[1] != 400 {
		t.Fatalf("content lengths %v", lens)
	}

	rec = doReq(mux, adminReq(t, http.MethodPost, "/admin/albums/"+created.ID+"/photos/"+slot.PhotoID+"/complete", `{"width":4000,"height":3000}`, secret))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "incomplete") {
		t.Fatalf("complete missing %d %s", rec.Code, rec.Body.String())
	}
	store.add(wantOriginal, wantWeb)
	rec = doReq(mux, adminReq(t, http.MethodPost, "/admin/albums/"+created.ID+"/photos/"+slot.PhotoID+"/complete", `{"width":4000,"height":3000}`, secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("complete %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, httptest.NewRequest(http.MethodGet, "/api/gallery/"+token, nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"locked":true`) {
		t.Fatalf("locked %d %s", rec.Code, rec.Body.String())
	}

	badPin := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"0000"}`))
	badPin.Header.Set("Content-Type", "application/json")
	badPin.RemoteAddr = "192.0.2.40:9"
	rec = doReq(mux, badPin)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad pin %d %s", rec.Code, rec.Body.String())
	}

	unlockReq := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"2468"}`))
	unlockReq.Header.Set("Content-Type", "application/json")
	unlockReq.RemoteAddr = "192.0.2.20:1234"
	rec = doReq(mux, unlockReq)
	if rec.Code != http.StatusOK {
		t.Fatalf("unlock %d %s", rec.Code, rec.Body.String())
	}
	var guestCookie *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == galleryCookieName {
			guestCookie = c
		}
	}
	if guestCookie == nil {
		t.Fatal("missing gallery cookie")
	}

	viewReq := httptest.NewRequest(http.MethodGet, "/api/gallery/"+token, nil)
	viewReq.AddCookie(guestCookie)
	rec = doReq(mux, viewReq)
	var viewed publicAlbumOut
	decodeBody(t, rec, &viewed)
	if viewed.Locked || len(viewed.Photos) != 1 || viewed.Photos[0].WebURL == "" {
		t.Fatalf("view %#v", viewed)
	}
	if store.lastGetTTL() != spacesViewTTL {
		t.Fatalf("view ttl %s", store.lastGetTTL())
	}

	for _, variant := range []string{"web", "original"} {
		dl := httptest.NewRequest(http.MethodGet, "/api/gallery/"+token+"/photos/"+slot.PhotoID+"/download?variant="+variant, nil)
		dl.AddCookie(guestCookie)
		rec = doReq(mux, dl)
		if rec.Code != http.StatusOK {
			t.Fatalf("download %s %d %s", variant, rec.Code, rec.Body.String())
		}
		var body struct {
			URL      string `json:"url"`
			Filename string `json:"filename"`
		}
		decodeBody(t, rec, &body)
		if variant == "web" && (body.Filename != "Day One.jpg" || !strings.Contains(body.URL, "web.jpg")) {
			t.Fatalf("web download %#v", body)
		}
		if variant == "original" && (body.Filename != "Day One.JPG" || !strings.Contains(body.URL, "original.jpg")) {
			t.Fatalf("original download %#v", body)
		}
	}
	if store.lastGetTTL() != spacesPresignTTL {
		t.Fatalf("download ttl %s", store.lastGetTTL())
	}

	noCookie := httptest.NewRequest(http.MethodGet, "/api/gallery/"+token+"/photos/"+slot.PhotoID+"/download?variant=print", nil)
	rec = doReq(mux, noCookie)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("download auth %d", rec.Code)
	}
	badVariant := httptest.NewRequest(http.MethodGet, "/api/gallery/"+token+"/photos/"+slot.PhotoID+"/download?variant=print", nil)
	badVariant.AddCookie(guestCookie)
	rec = doReq(mux, badVariant)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("variant %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, adminReq(t, http.MethodPatch, "/admin/albums/"+created.ID, `{"pin":"1357"}`, secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("patch %d %s", rec.Code, rec.Body.String())
	}

	rec = doReq(mux, adminReq(t, http.MethodDelete, "/admin/albums/"+created.ID+"/photos/"+slot.PhotoID, "", secret))
	if rec.Code != http.StatusOK || store.has(wantOriginal) || store.has(wantWeb) {
		t.Fatalf("delete photo %d store orig %v", rec.Code, store.has(wantOriginal))
	}

	rec = doReq(mux, adminReq(t, http.MethodDelete, "/admin/albums/"+created.ID, "", secret))
	if rec.Code != http.StatusOK {
		t.Fatalf("delete album %d %s", rec.Code, rec.Body.String())
	}
	rec = doReq(mux, httptest.NewRequest(http.MethodGet, "/api/gallery/"+token, nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("gone %d %s", rec.Code, rec.Body.String())
	}
}

func TestGalleryUnlockRateLimit(t *testing.T) {
	t.Parallel()
	_, mux, secret := newGalleryMux(t, newMemStore(), 2)
	rec := doReq(mux, adminReq(t, http.MethodPost, "/admin/albums", `{"title":"Limit","pin":"1234"}`, secret))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created albumOut
	decodeBody(t, rec, &created)
	token := strings.TrimPrefix(created.SharePath, "/gallery/")

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"9999"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "198.51.100.8:9"
		rec = doReq(mux, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d %s", i, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"1234"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "198.51.100.8:9"
	rec = doReq(mux, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("limit %d %s", rec.Code, rec.Body.String())
	}
}

func TestRateLimitClientIP(t *testing.T) {
	t.Setenv("TRUST_PROXY", "")
	direct := httptest.NewRequest(http.MethodGet, "/", nil)
	direct.RemoteAddr = "192.0.2.8:1234"
	direct.Header.Set("X-Forwarded-For", "198.51.100.1, 203.0.113.9")
	if got := rateLimitClientIP(direct); got != "192.0.2.8" {
		t.Fatalf("direct %s", got)
	}

	t.Setenv("TRUST_PROXY", "1")
	if got := rateLimitClientIP(direct); got != "203.0.113.9" {
		t.Fatalf("last hop %s", got)
	}
	blank := httptest.NewRequest(http.MethodGet, "/", nil)
	blank.RemoteAddr = "192.0.2.8:1234"
	blank.Header.Set("X-Forwarded-For", "198.51.100.1, ")
	if got := rateLimitClientIP(blank); got != "198.51.100.1" {
		t.Fatalf("skip blank hop %s", got)
	}
}

func TestGalleryUnlockSharesLastForwardedHop(t *testing.T) {
	t.Setenv("TRUST_PROXY", "1")
	_, mux, secret := newGalleryMux(t, newMemStore(), 2)
	rec := doReq(mux, adminReq(t, http.MethodPost, "/admin/albums", `{"title":"Limit","pin":"1234"}`, secret))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d %s", rec.Code, rec.Body.String())
	}
	var created albumOut
	decodeBody(t, rec, &created)
	token := strings.TrimPrefix(created.SharePath, "/gallery/")

	for i, first := range []string{"198.51.100.1", "198.51.100.2"} {
		req := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"9999"}`))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "127.0.0.1:9"
		req.Header.Set("X-Forwarded-For", first+", 203.0.113.9")
		rec = doReq(mux, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d status %d %s", i, rec.Code, rec.Body.String())
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/api/gallery/"+token+"/unlock", strings.NewReader(`{"pin":"1234"}`))
	req.Header.Set("Content-Type", "application/json")
	req.RemoteAddr = "127.0.0.1:9"
	req.Header.Set("X-Forwarded-For", "198.51.100.3, 203.0.113.9")
	rec = doReq(mux, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("shared hop %d %s", rec.Code, rec.Body.String())
	}
}

func TestCookieSameSite(t *testing.T) {
	t.Setenv("APP_ENV", "")
	t.Setenv("CORS_ORIGIN", "https://www.example.com")
	if cookieSameSite() != http.SameSiteLaxMode {
		t.Fatal("dev stays lax")
	}
	t.Setenv("APP_ENV", "production")
	t.Setenv("CORS_ORIGIN", "")
	if cookieSameSite() != http.SameSiteLaxMode {
		t.Fatal("same-origin production stays lax")
	}
	t.Setenv("CORS_ORIGIN", "https://www.example.com")
	if cookieSameSite() != http.SameSiteNoneMode {
		t.Fatal("cross-origin production uses none")
	}
}
