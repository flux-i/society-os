package server

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestPublicBuildCachingKeepsPrivateAndMutableResponsesNoStore(t *testing.T) {
	web := fstest.MapFS{
		"index.html":                 {Data: []byte("<!doctype html><title>Public shell</title>")},
		"sw.js":                      {Data: []byte("/* public worker */")},
		"manifest.webmanifest":       {Data: []byte(`{"name":"Society"}`)},
		"icon.svg":                   {Data: []byte("<svg/>")},
		"assets/index-Abc123_-.js":   {Data: []byte("/* public versioned script */")},
		"assets/style-Abc123_-.css":  {Data: []byte("body {}")},
		"assets/font-Abc123_-.woff2": {Data: []byte("public font")},
		"assets/private.json":        {Data: []byte(`{"private":true}`)},
	}
	h := (&Server{Web: web, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}).Handler()
	for _, item := range []struct {
		path      string
		code      int
		immutable bool
	}{
		{"/", 200, false}, {"/sw.js", 200, false}, {"/manifest.webmanifest", 200, false}, {"/icon.svg", 200, false},
		{"/assets/index-Abc123_-.js", 200, true}, {"/assets/style-Abc123_-.css", 200, true}, {"/assets/font-Abc123_-.woff2", 200, true},
		{"/assets/index-Abc123_-.js?token=synthetic", 200, false}, {"/assets/missing-Abc123_-.js", 404, false},
		{"/assets/private.json", 200, false}, {"/api/not-a-static-file-Abc123_-.js", 404, false},
	} {
		t.Run(item.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:8080"+item.path, nil))
			if w.Code != item.code {
				t.Fatalf("status %d, want %d", w.Code, item.code)
			}
			if item.path == "/manifest.webmanifest" && w.Header().Get("Content-Type") != "application/manifest+json; charset=utf-8" {
				t.Fatalf("manifest MIME: %q", w.Header().Get("Content-Type"))
			}
			want := "no-store"
			if item.immutable {
				want = "public, max-age=31536000, immutable"
			}
			if got := w.Header().Get("Cache-Control"); got != want {
				t.Fatalf("cache %q, want %q", got, want)
			}
		})
	}
}
