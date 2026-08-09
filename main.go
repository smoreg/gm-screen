// Command gm-screen serves the GM-экран static app plus a small API that turns
// pasted character text into the canonical gm-character/v1 JSON via Claude.
//
// Config (env):
//   PORT                 HTTP port (default 8777)
//   GM_WEB_DIR           static dir to serve (default ./web)
//   ANTHROPIC_API_KEY    required for /api/parse-character (SDK reads it)
//   PARSE_MODEL          Claude model id (default claude-opus-4-8)
//
// Secrets never live in the repo — load them from a gitignored env file, e.g.
//   set -a; . ./secrets/env; set +a; ./gm-screen
package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	port := env("PORT", "8777")
	webDir := env("GM_WEB_DIR", "./web")

	if _, err := os.Stat(filepath.Join(webDir, "index.html")); err != nil {
		log.Printf("warning: %s/index.html not found — static app will 404", webDir)
	}

	mux := http.NewServeMux()

	// Health check — cheap, no external deps.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	// Character parser: paste text -> gm-character/v1 JSON.
	mux.HandleFunc("POST /api/parse-character", handleParseCharacter)

	// Static GM-экран (index.html, docs/, character.schema.json, …).
	fs := http.FileServer(http.Dir(webDir))
	mux.Handle("/", noCacheHTML(fs))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           logRequests(cors(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		// Character parsing can take a while — allow a generous write timeout.
		WriteTimeout: 90 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("gm-screen listening on http://127.0.0.1:%s (web: %s)", port, webDir)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// noCacheHTML keeps .html from getting stale in the browser between redeploys,
// while letting static assets cache normally.
func noCacheHTML(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if filepath.Ext(r.URL.Path) == ".html" || r.URL.Path == "/" {
			w.Header().Set("Cache-Control", "no-cache")
		}
		next.ServeHTTP(w, r)
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Truncate(time.Millisecond))
	})
}
