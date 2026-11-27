package output

import (
	"fmt"
	"io"

	"github.com/bananeadin-code/saasift/internal/scanner"
)

const (
	reset  = "\033[om"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	purple = "\033[35m"
	cyan   = "\033[36m"
)

type Console struct {
	w      io.Writer
	colors bool
}

func NewConsole(w io.Writer, colors bool) *Console {
	return &Console{w: w, colors: colors}
}

func (c *Console) Report(r scanner.Result) error {
	target := "/" + r.Path
	if r.Location != "" {
		target += " -> " + r.Location
	}

	line := fmt.Sprintf("[%d] %-45s %8dB %6dW %5dL  %s", r.Status, target, r.Size, r.Words, r.Lines, r.ContentType)
	if r.Filtered != "" {
		line += "	(filtered: " + r.Filtered + ")"
	}

	color := statusColor(r.Status)
	if r.Filtered != "" {
		color = dim
	}

	_, err := fmt.Fprintln(c.w, c.paint(color, line))
	return err
}

func (c *Console) Close() error { return nil }

func (c *Console) paint(color, text string) string {
	if !c.colors {
		return text
	}
	return color + text + reset
}

func statusColor(status int) string {
	switch {
	case status >= 200 && status < 300:
		return green
	case status >= 300 && status < 400:
		return cyan
	case status == 401 || status == 403:
		return yellow
	case status >= 500:
		return purple
	default:
		return blue
	}
}
