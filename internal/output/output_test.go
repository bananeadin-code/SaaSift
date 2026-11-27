package output

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bananeadin-code/saasift/internal/scanner"
)

var sample = scanner.Result{
	Path: "robots.txt", Status: 200, Size: 293, Words: 34, Lines: 12,
	ContentType: "text/plain",
}

func TestConsole_NoColors(t *testing.T) {
	var buf bytes.Buffer
	c := NewConsole(&buf, false)

	if err := c.Report(sample); err != nil {
		t.Fatal(err)
	}

	got := buf.String()
	for _, want := range []string{"[200]", "/robots.txt", "293B", "34W", "12L", "text/plain"} {
		if !strings.Contains(got, want) {
			t.Errorf("la salida %q no contiene %q", got, want)
		}
	}
	if strings.Contains(got, "\033[") {
		t.Errorf("sin colores no debería haber códigos ANSI: %q", got)
	}
}

func TestConsole_Colors(t *testing.T) {
	var buf bytes.Buffer
	NewConsole(&buf, true).Report(sample)
	if !strings.HasPrefix(buf.String(), green) {
		t.Errorf("un 200 debería ir en verde: %q", buf.String())
	}
}

func TestJSON(t *testing.T) {
	var buf bytes.Buffer
	j := NewJSON(&buf, "https://example.com")
	j.Report(sample)
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	var got jsonReport
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("JSON inválido: %v\n%s", err, buf.String())
	}
	if got.Target != "https://example.com" || len(got.Results) != 1 {
		t.Fatalf("reporte inesperado: %+v", got)
	}
	r := got.Results[0]
	if r.Path != "/robots.txt" || r.URL != "https://example.com/robots.txt" || r.Status != 200 {
		t.Errorf("resultado inesperado: %+v", r)
	}
	if strings.Contains(buf.String(), `"location"`) {
		t.Error("omitempty: location vacío no debería aparecer")
	}
}

func TestJSON_Empty(t *testing.T) {
	var buf bytes.Buffer
	NewJSON(&buf, "https://example.com").Close()
	if !strings.Contains(buf.String(), `"results": []`) {
		t.Errorf("sin resultados debería ser [] y no null:\n%s", buf.String())
	}
}

func TestCSV(t *testing.T) {
	var buf bytes.Buffer
	c, err := NewCSV(&buf)
	if err != nil {
		t.Fatal(err)
	}
	c.Report(sample)
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("CSV inválido: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("esperaba encabezado + 1 fila, obtuve %d filas", len(rows))
	}
	if rows[1][0] != "/robots.txt" || rows[1][1] != "200" {
		t.Errorf("fila inesperada: %v", rows[1])
	}
}

type fakeReporter struct {
	got  int
	fail bool
}

func (f *fakeReporter) Report(r scanner.Result) error {
	f.got++
	if f.fail {
		return errors.New("disco lleno")
	}
	return nil
}
func (f *fakeReporter) Close() error { return nil }

func TestMulti(t *testing.T) {
	a, b := &fakeReporter{}, &fakeReporter{fail: true}
	m := Multi{a, b}

	err := m.Report(sample)
	if a.got != 1 || b.got != 1 {
		t.Errorf("ambos reporters deberían recibir el resultado: a=%d b=%d", a.got, b.got)
	}
	if err == nil {
		t.Error("el error de b debería propagarse")
	}
}

func TestStatsSummary(t *testing.T) {
	s := Stats{Processed: 209, Found: 4, Filtered: 205, Start: time.Now().Add(-2 * time.Second)}
	got := s.Summary()
	if !strings.Contains(got, "209 requests | 4 found | 205 filtered | 0 errors") {
		t.Errorf("resumen inesperado: %q", got)
	}
}
