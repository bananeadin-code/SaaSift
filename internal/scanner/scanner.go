package scanner

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

const DefaultUserAgent = "SaaSift/0.1 (+https://github.com/bananeadin-code/saasift)"

type Config struct {
	BaseURL   string
	Threads   int
	Retries   int
	UserAgent string
	Headers   http.Header
	Rate      int
}

type Result struct {
	Path        string
	Status      int
	Size        int
	Words       int
	Lines       int
	ContentType string
	Location    string
	Err         error
}

const maxBodySize = 512 * 1024

type Scanner struct {
	cfg     Config
	client  *http.Client
	limiter <-chan time.Time
}

func New(cfg Config, client *http.Client) *Scanner {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &Scanner{cfg: cfg, client: client}
}

func (s *Scanner) Run(ctx context.Context, wordlist io.Reader) <-chan Result {
	paths := make(chan string)
	results := make(chan Result)

	var ticker *time.Ticker
	if s.cfg.Rate > 0 {
		ticker = time.NewTicker(time.Second / time.Duration(s.cfg.Rate))
		s.limiter = ticker.C
	}

	var wg sync.WaitGroup
	for i := 0; i < s.cfg.Threads; i++ {
		wg.Add(1)
		go s.worker(ctx, paths, results, &wg)
	}

	go func() {
		defer close(paths)
		sc := bufio.NewScanner(wordlist)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			select {
			case paths <- strings.TrimLeft(line, "/"):
			case <-ctx.Done():
				return
			}
		}

		if err := sc.Err(); err != nil {
			select {
			case results <- Result{Err: fmt.Errorf("reading wordlist: %w", err)}:
			case <-ctx.Done():
			}
		}
	}()

	go func() {
		wg.Wait()
		if ticker != nil {
			ticker.Stop()
		}
		close(results)
	}()

	return results
}

func (s *Scanner) worker(ctx context.Context, paths <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range paths {
		r, err := s.checkWithRetry(ctx, s.cfg.BaseURL+"/"+path)
		r.Path = path
		r.Err = err
		results <- r
	}
}

func (s *Scanner) waitTurn(ctx context.Context) error {
	if s.limiter == nil {
		return nil
	}
	select {
	case <-s.limiter:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *Scanner) checkWithRetry(ctx context.Context, fullURL string) (Result, error) {
	backoff := 500 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt <= s.cfg.Retries; attempt++ {
		if err := s.waitTurn(ctx); err != nil {
			return Result{}, err
		}

		r, err := s.checkURL(ctx, fullURL)
		if err == nil {
			return r, nil
		}
		lastErr = err

		if attempt == s.cfg.Retries || ctx.Err() != nil {
			break
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return Result{}, ctx.Err()
		}
		backoff *= 2
	}
	return Result{}, lastErr
}

func (s *Scanner) checkURL(ctx context.Context, fullURL string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	for name, values := range s.cfg.Headers {
		for _, v := range values {
			req.Header.Set(name, v)
		}
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return Result{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return Result{}, err
	}

	return Result{
		Status:      resp.StatusCode,
		Size:        len(body),
		Words:       len(bytes.Fields(body)),
		Lines:       countLines(body),
		ContentType: resp.Header.Get("Content-Type"),
		Location:    resp.Header.Get("Location"),
	}, nil
}

func countLines(body []byte) int {
	if len(body) == 0 {
		return 0
	}
	n := bytes.Count(body, []byte("\n"))
	if body[len(body)-1] != '\n' {
		n++
	}
	return n
}
