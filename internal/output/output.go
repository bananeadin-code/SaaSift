package output

import (
	"errors"
	"os"

	"github.com/bananeadin-code/saasift/internal/scanner"
)

type Reporter interface {
	Report(r scanner.Result) error
	Close() error
}

type Multi []Reporter

func (m Multi) Report(r scanner.Result) error {
	var errs []error
	for _, rep := range m {
		if err := rep.Report(r); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (m Multi) Close() error {
	var errs []error
	for _, rep := range m {
		if err := rep.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
