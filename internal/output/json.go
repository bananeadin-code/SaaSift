package output

import (
	"encoding/json"
	"io"
	"time"

	"github.com/bananeadin-code/saasift/internal/scanner"
)

type jsonResult struct {
	Path        string `json:"path"`
	URL         string `json:"url"`
	Status      int    `json:"status"`
	Size        int    `json:"size"`
	Words       int    `json:"words"`
	Lines       int    `json:"lines"`
	ContentType string `json:"content_type,omitempty"`
	Location    string `json:"location,omitempty"`
	Filtered    string `json:"filtered,omitempty"`
}

type jsonReport struct {
	Tool    string       `json:"tool"`
	Target  string       `json:"target"`
	Date    time.Time    `json:"date"`
	Results []jsonResult `json:"results"`
}

type JSON struct {
	w      io.Writer
	report jsonReport
}

func NewJSON(w io.Writer, target string) *JSON {
	return &JSON{
		w: w,
		report: jsonReport{
			Tool:    "saasift",
			Target:  target,
			Date:    time.Now().UTC(),
			Results: []jsonResult{},
		},
	}
}

func (j *JSON) Report(r scanner.Result) error {
	j.report.Results = append(j.report.Results, jsonResult{
		Path:        "/" + r.Path,
		URL:         j.report.Target + "/" + r.Path,
		Status:      r.Status,
		Size:        r.Size,
		Words:       r.Words,
		Lines:       r.Lines,
		ContentType: r.ContentType,
		Location:    r.Location,
		Filtered:    r.Filtered,
	})
	return nil
}

func (j *JSON) Close() error {
	enc := json.NewEncoder(j.w)
	enc.SetIndent("", " ")
	return enc.Encode(j.report)
}
