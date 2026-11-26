package scanner

import (
	"context"
	"crypto/rand"
	"mime"
	"path"
	"strings"
	"sync"
)

type fingerprint struct {
	status      int
	contentType string
	size        int
	words       int
	lines       int
}

func fingerprintOf(r Result) fingerprint {
	return fingerprint{
		status:      r.Status,
		contentType: r.ContentType,
		size:        r.Size,
		words:       r.Words,
		lines:       r.Lines,
	}
}

func (f fingerprint) matches(r Result) bool {
	if f.status != r.Status || f.contentType != r.ContentType {
		return false
	}
	return f.size == r.Size || (f.words == r.Words && f.lines == r.Lines)
}

func shapeOf(p string) (dir, ext string) {
	p = strings.Trim(p, "/")
	return path.Dir(p), strings.ToLower(path.Ext(path.Base(p)))
}

type calibration struct {
	mu     sync.Mutex
	shapes map[string]*shapeEntry
}

type shapeEntry struct {
	once sync.Once
	fp   *fingerprint
}

func newCalibration() *calibration {
	return &calibration{shapes: map[string]*shapeEntry{}}
}

func (c *calibration) entry(key string) *shapeEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.shapes[key]
	if !ok {
		e = &shapeEntry{}
		c.shapes[key] = e
	}
	return e
}

func (s *Scanner) isSoft404(ctx context.Context, p string, r Result) bool {
	dir, ext := shapeOf(p)
	e := s.cal.entry(dir + "|" + ext)

	e.once.Do(func() {
		probe := strings.ToLower(rand.Text()) + ext
		if dir != "." {
			probe = dir + "/" + probe
		}
		cr, err := s.checkWithRetry(ctx, s.cfg.BaseURL+"/"+probe)
		if err == nil {
			fp := fingerprintOf(cr)
			e.fp = &fp
		}
	})

	return e.fp != nil && e.fp.matches(r)
}

var htmlExts = map[string]bool{
	"": true, ".html": true, ".htm": true, ".xhtml": true, ".shtml": true,
	".php": true, ".asp": true, ".aspx": true, ".jsp": true,
}

func contentTypeMismatch(p string, r Result) bool {
	if r.Status != 200 {
		return false
	}
	_, ext := shapeOf(p)
	return !htmlExts[ext] && r.ContentType == "text/html"
}

func mediaType(contentType string) string {
	mt, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		mt, _, _ = strings.Cut(contentType, ";")
	}
	return strings.ToLower(strings.TrimSpace(mt))
}
