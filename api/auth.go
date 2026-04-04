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
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionCookieName = "admin_session"

type sessionPayload struct {
	Exp int64 `json:"exp"`
}

func verifyAdminPassword(hashFromEnv, plain string) bool {
	if hashFromEnv == "" || plain == "" {
		return false
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashFromEnv), []byte(plain))
	return err == nil
}

func signSession(secret []byte, exp time.Time) (string, error) {
	p := sessionPayload{Exp: exp.Unix()}
	body, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	sig := mac.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(body) + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func parseSession(secret []byte, raw string) error {
	parts := strings.Split(raw, ".")
	if len(parts) != 2 {
		return errors.New("invalid session")
	}
	body, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, secret)
	mac.Write(body)
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return errors.New("bad signature")
	}
	var p sessionPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return err
	}
	if time.Now().Unix() > p.Exp {
		return errors.New("expired")
	}
	return nil
}

func readSession(r *http.Request, secret []byte) error {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return err
	}
	return parseSession(secret, c.Value)
}

func setSessionCookie(w http.ResponseWriter, secret []byte, exp time.Time) error {
	val, err := signSession(secret, exp)
	if err != nil {
		return err
	}
	sameSite := http.SameSiteLaxMode
	secure := isProd()
	if secure {
		// Admin UI on another origin (e.g. www) must send this cookie on credentialed fetches to the API host.
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    val,
		Path:     "/",
		MaxAge:   int(time.Until(exp).Seconds()),
		HttpOnly: true,
		SameSite: sameSite,
		Secure:   secure,
	})
	return nil
}

func clearSessionCookie(w http.ResponseWriter) {
	sameSite := http.SameSiteLaxMode
	secure := isProd()
	if secure {
		sameSite = http.SameSiteNoneMode
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: sameSite,
		Secure:   secure,
	})
}

func isProd() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	return v == "production" || v == "prod"
}

func requireAdmin(secret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := readSession(r, secret); err != nil {
			slog.DebugContext(r.Context(), "admin unauthorized",
				"path", r.URL.Path,
				"method", r.Method,
				"remote_addr", requestRemoteAddr(r),
				"reason", err.Error(),
			)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}
