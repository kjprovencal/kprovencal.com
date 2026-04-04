package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	initLogging()

	addr := strings.TrimSpace(os.Getenv("LISTEN_ADDR"))
	if addr == "" {
		addr = ":8080"
	}
	dbPath := strings.TrimSpace(os.Getenv("BADGER_PATH"))
	if dbPath == "" {
		dbPath = filepath.Join("data", "badger")
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("data directory: %v", err)
	}

	sessionSecret := []byte(strings.TrimSpace(os.Getenv("SESSION_SECRET")))
	if len(sessionSecret) < 32 {
		log.Fatal("SESSION_SECRET must be set to at least 32 bytes (use a long random string)")
	}

	passwordHash := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD_BCRYPT"))
	if passwordHash == "" {
		log.Fatal("ADMIN_PASSWORD_BCRYPT must be set (bcrypt hash of your admin password)")
	}

	db, err := openDB(dbPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	cors := strings.TrimSpace(os.Getenv("CORS_ORIGIN"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	mountPublicAPI(mux, db, cors)
	mountAdminRoutes(mux, db, sessionSecret, passwordHash, cors)

	slog.Info("server starting",
		"listen_addr", addr,
		"badger_path", dbPath,
		"cors_enabled", cors != "",
	)
	handler := withRequestLogging(mux)
	if err := http.ListenAndServe(addr, handler); err != nil {
		slog.Error("server exit", "err", err)
		log.Fatal(err)
	}
}
