package output

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/bananeadin-code/saasift/internal/scanner"
)

type CSV struct {
	w *csv.Writer
}

func NewCSV(w io.Writer) (*CSV, error) {
	c := &CSV{w: csv.NewWriter(w)}
	header := []string{"path", "status", "size", "words", "lines", "content_type", "location", "filtered"}
	if err := c.w.Write(header); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *CSV) Report(r scanner.Result) error {
	return c.w.Write([]string{
		"/" + r.Path,
		strconv.Itoa(r.Status),
		strconv.Itoa(r.Size),
		strconv.Itoa(r.Words),
		strconv.Itoa(r.Lines),
		r.ContentType,
		r.Location,
		r.Filtered,
	})
}

func (c *CSV) Close() error {
	c.w.Flush()
	return c.w.Error()
}
