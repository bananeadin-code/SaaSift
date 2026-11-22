package scanner

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()

	mux.HandleFunc("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "test/plain")
		w.Write([]byte("User-agent: *\nDisallow: /secreto\n"))
	})
	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func newTestScanner(baseURL string, retries int) *Scanner {
	client := &http.Client{
		Timeout: 2 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return New(Config{
		BaseURL:   baseURL,
		Threads:   3,
		Retries:   retries,
		UserAgent: DefaultUserAgent,
	}, client)
}

func TestRun(t *testing.T) {
	srv := newTestServer(t)
	s := newTestScanner(srv.URL, 0)

	wordlist := strings.NewReader("# comentario\nrobots.txt\n\n/admin\nno-existe\n")

	got := map[string]Result{}
	for r := range s.Run(context.Background(), wordlist) {
		if r.Err != nil {
			t.Fatalf("error inesperado en %q: %v", r.Path, r.Err)
		}
		got[r.Path] = r
	}

	tests := []struct {
		path     string
		status   int
		location string
		words    int
		lines    int
	}{
		{path: "robots.txt", status: 200, words: 4, lines: 2},
		{path: "admin", status: 301, location: "/admin/"},
		{path: "no-existe", status: 404},
	}

	if len(got) != len(tests) {
		t.Fatalf("esperaba %d resultados, obtuve %d: %v", len(tests), len(got), got)
	}

	for _, tt := range tests {
		r, ok := got[tt.path]
		if !ok {
			t.Errorf("falta el resultado de %q", tt.path)
			continue
		}
		if r.Status != tt.status {
			t.Errorf("%s: status = %d, esperaba %d", tt.path, r.Status, tt.status)
		}
		if r.Location != tt.location {
			t.Errorf("%s: location = %q, esperaba %q", tt.path, r.Location, tt.location)
		}
		if tt.words > 0 && r.Words != tt.words {
			t.Errorf("%s: words = %d, esperaba %d", tt.path, r.Words, tt.words)
		}
		if tt.lines > 0 && r.Lines != tt.lines {
			t.Errorf("%s: lines = %d, esperaba %d", tt.path, r.Lines, tt.lines)
		}
	}
}

func flakyServer(t *testing.T, failures int32) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= failures {
			conn, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Errorf("hijack: %v", err)
				return
			}
			conn.Close()
			return
		}
		w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func TestCheckWithRetry_RecoversAfterFailures(t *testing.T) {
	srv, calls := flakyServer(t, 2)
	s := newTestScanner(srv.URL, 2)

	r, err := s.checkWithRetry(context.Background(), srv.URL+"/x")
	if err != nil {
		t.Fatalf("esperaba éxito tras reintentar, obtuve error: %v", err)
	}
	if r.Status != 200 {
		t.Errorf("status = %d, esperaba 200", r.Status)
	}
	if got := calls.Load(); got != 3 {
		t.Errorf("el servidor recibió %d peticiones, esperaba 3", got)
	}
}

func TestCheckWithRetry_GivesUp(t *testing.T) {
	srv, calls := flakyServer(t, 100)
	s := newTestScanner(srv.URL, 1)

	_, err := s.checkWithRetry(context.Background(), srv.URL+"/x")
	if err == nil {
		t.Fatal("esperaba un error, pero no hubo")
	}
	if got := calls.Load(); got != 2 {
		t.Errorf("el servidor recibió %d peticiones, esperaba 2", got)
	}
}

func TestCountLines(t *testing.T) {
	tests := []struct {
		name string
		body string
		want int
	}{
		{"vacío", "", 0},
		{"una línea sin salto", "hola", 1},
		{"una línea con salto", "hola\n", 1},
		{"dos líneas", "hola\nmundo", 2},
		{"dos líneas con salto final", "hola\nmundo\n", 2},
	}
	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			if got := countLines([]byte(tt.body)); got != tt.want {
				t.Errorf("countLines(%q) = %d, esperaba %d", tt.body, got, tt.want)
			}
		})
	}
}
