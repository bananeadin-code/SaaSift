package output

import (
	"fmt"
	"io"
	"strings"
	"time"
)

type Stats struct {
	Total     int
	Processed int
	Found     int
	Filtered  int
	Errors    int
	Start     time.Time
}

func (s Stats) Elapsed() time.Duration {
	return time.Since(s.Start)
}

func (s Stats) Rate() float64 {
	secs := s.Elapsed().Seconds()
	if secs == 0 {
		return 0
	}
	return float64(s.Processed) / secs
}

func (s Stats) Summary() string {
	return fmt.Sprintf("Done: %d requests | %d found | %d filtered | %d errors | %s (%.1f req/s)",
		s.Processed, s.Found, s.Filtered, s.Errors,
		s.Elapsed().Round(100*time.Millisecond), s.Rate())
}

type Progress struct {
	w        io.Writer
	enabled  bool
	lastDraw time.Time
}

func NewProgress(w io.Writer, enabled bool) *Progress {
	return &Progress{w: w, enabled: enabled}
}

func (p *Progress) Draw(s Stats, force bool) {
	if !p.enabled || (!force && time.Since(p.lastDraw) < 100*time.Millisecond) {
		return
	}
	p.lastDraw = time.Now()

	const width = 25
	done := 0
	if s.Total > 0 {
		done = s.Processed * width / s.Total
	}
	done = min(done, width)

	bar := strings.Repeat("=", done) + strings.Repeat(" ", width-done)

	fmt.Fprintf(p.w, "\r\033[K[%s] %d/%d | %.0f req/s | %d found | %d err",
		bar, s.Processed, s.Total, s.Rate(), s.Found, s.Errors)

}

func (p *Progress) Clear() {
	if p.enabled {
		fmt.Fprint(p.w, "\r\033[K")
	}
}
