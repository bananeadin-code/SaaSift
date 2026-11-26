package scanner

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync/atomic"
	"testing"
)

func spaServer(t *testing.T) *httptest.Server {
	t.Helper()
	var n atomic.Int64

	indexHTML := func() string {
		return fmt.Sprintf("<!doctype html>\n<html>\n<body>\n<div id=\"root\"></div>\n<!-- req %d -->\n</body>\n</html>\n", n.Add(7))
	}
	const s3Denied = "<?xml version=\"1.0\"?>\n<Error><Code>AccessDenied</Code><Message>Access Denied</Message></Error>"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		switch {
		case p == "/robots.txt":
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "User-agent: *\nDisallow: /private\n")
		case p == "/login":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, "<!doctype html>\n<html>\n<body>\n<h1>Iniciar sesión</h1>\n<form>usuario y contraseña</form>\n</body>\n</html>\n")
		case p == "/app.js":
			w.Header().Set("Content-Type", "application/javascript")
			fmt.Fprint(w, "console.log('app')\n")
		case p == "/backup.zip":
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, "<html><body>Oops, no encontramos eso</body></html>")
		case strings.HasPrefix(p, "/api/"):
			http.Redirect(w, r, "https://api.example.com"+p, http.StatusFound)
		case path.Ext(p) == "":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprint(w, indexHTML())
		default:
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, s3Denied)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func scanAll(t *testing.T, s *Scanner, wordlist string) map[string]Result {
	t.Helper()
	got := map[string]Result{}
	for r := range s.Run(context.Background(), strings.NewReader(wordlist)) {
		if r.Err != nil {
			t.Fatalf("error inesperado en %q: %v", r.Path, r.Err)
		}
		got[r.Path] = r
	}
	return got
}

const spaWordlist = `robots.txt
login
app.js
backup.zip
admin
dashboard
.git/HEAD
.env
config.json
api/v1
api/admin
`

func TestCalibration_SPA(t *testing.T) {
	srv := spaServer(t)
	s := newTestScanner(srv.URL, 0)
	s.cfg.Calibrate = true

	got := scanAll(t, s, spaWordlist)

	want := map[string]string{
		"robots.txt":  "",
		"login":       "",
		"app.js":      "",
		"backup.zip":  "content-type",
		"admin":       "calibration",
		"dashboard":   "calibration",
		".git/HEAD":   "calibration",
		".env":        "calibration",
		"config.json": "calibration",
		"api/v1":      "calibration",
		"api/admin":   "calibration",
	}

	for p, reason := range want {
		r, ok := got[p]
		if !ok {
			t.Errorf("falta el resultado de %q", p)
			continue
		}
		if r.Filtered != reason {
			t.Errorf("%s: Filtered = %q, esperaba %q (status %d, size %d, type %s)",
				p, r.Filtered, reason, r.Status, r.Size, r.ContentType)
		}
	}
}

func TestCalibration_Disabled(t *testing.T) {
	srv := spaServer(t)
	s := newTestScanner(srv.URL, 0)
	s.cfg.Calibrate = false

	got := scanAll(t, s, spaWordlist)

	for p, r := range got {
		want := ""
		if p == "backup.zip" {
			want = "content-type"
		}
		if r.Filtered != want {
			t.Errorf("%s: Filtered = %q, esperaba %q", p, r.Filtered, want)
		}
	}
}

func TestShapeOf(t *testing.T) {
	tests := []struct {
		path    string
		wantDir string
		wantExt string
	}{
		{"admin", ".", ""},
		{"/admin/", ".", ""},
		{".env", ".", ".env"},
		{"config.JSON", ".", ".json"},
		{".git/HEAD", ".git", ""},
		{"Dockerfile", ".", ""},
		{"static/js/main.js.map", "static/js", ".map"},
		{"api/v1", "api", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			dir, ext := shapeOf(tt.path)
			if dir != tt.wantDir || ext != tt.wantExt {
				t.Errorf("shapeOf(%q) = (%q, %q), esperaba (%q, %q)",
					tt.path, dir, ext, tt.wantDir, tt.wantExt)
			}
		})
	}
}

func TestMediaType(t *testing.T) {
	tests := map[string]string{
		"text/html; charset=utf-8":        "text/html",
		"application/json; charset=UTF-8": "application/json",
		"TEXT/PLAIN":                      "text/plain",
		"":                                "",
	}
	for in, want := range tests {
		if got := mediaType(in); got != want {
			t.Errorf("mediaType(%q) = %q, esperaba %q", in, got, want)
		}
	}
}
