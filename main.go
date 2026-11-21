package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

const defaultUserAgent = "SaaSift/0.1 (+https://github.com/bananeadin-code/saasift)"

type result struct {
	path     string
	status   int
	location string
	err      error
}

func newClient(threads int, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = threads
	transport.MaxIdleConnsPerHost = threads

	return &http.Client{
		Transport: transport,
		Timeout:   timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func checkURL(ctx context.Context, client *http.Client, fullURL, userAgent string) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return 0, "", err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil {
		return 0, "", err
	}

	defer resp.Body.Close()

	io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))

	return resp.StatusCode, resp.Header.Get("Location"), nil
}

func checkWithRetry(ctx context.Context, client *http.Client, fullURL, userAgent string,
	retries int) (int, string, error) {

	backoff := 500 * time.Millisecond
	var lastErr error

	for attempt := 0; attempt <= retries; attempt++ {
		status, location, err := checkURL(ctx, client, fullURL, userAgent)
		if err == nil {
			return status, location, nil
		}
		lastErr = err

		if attempt == retries || ctx.Err() != nil {
			break
		}

		select {
		case <-time.After(backoff):
		case <-ctx.Done():
			return 0, "", ctx.Err()
		}
		backoff *= 2
	}
	return 0, "", lastErr

}

func worker(ctx context.Context, client *http.Client, base, userAgent string,
	retries int, paths <-chan string, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	for path := range paths {
		status, location, err := checkWithRetry(ctx, client, base+"/"+path, userAgent, retries)
		results <- result{path: path, status: status, location: location, err: err}
	}
}

func main() {

	// Definir los flags, es decir, "parámetros" de la ejecución
	target := flag.String("u", "", "Target URL")
	wordlist := flag.String("w", "", "Dictionary path, ej: test.txt")
	threads := flag.Int("t", 10, "Number of concurrent workers")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout per request, e.g. 5s, 500ms")
	retries := flag.Int("retries", 2, "Retries on network errors or timeouts")
	userAgent := flag.String("a", defaultUserAgent, "User-Agent header")
	flag.Parse()

	if *target == "" || *wordlist == "" {
		fmt.Println("Error: You must indicate -u and -w")
		flag.Usage()
		os.Exit(1)
	}

	if *threads < 1 {
		fmt.Println("Error: -t must be at least 1")
		os.Exit(1)
	}

	if *retries < 0 {
		fmt.Println("Error: -retries cannot be negative")
	}

	file, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println("Error: Can't open the specified dictionary:", err)
		os.Exit(1)
	}

	defer file.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Quitamos / para evitar mal formato
	base := strings.TrimRight(*target, "/")

	client := newClient(*threads, *timeout)

	paths := make(chan string)
	results := make(chan result)

	var wg sync.WaitGroup

	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go worker(ctx, client, base, *userAgent, *retries, paths, results, &wg)
	}

	go func() {
		defer close(paths)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			select {
			case paths <- strings.TrimLeft(line, "/"):
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading wordlist:", err)
		}
	}()

	go func() {
		wg.Wait()

		close(results)
	}()

	for r := range results {
		if r.err != nil {
			if errors.Is(r.err, context.Canceled) {
				continue
			}
			fmt.Println("[ERR]", r.path, r.err)
			continue
		}
		if r.status == http.StatusNotFound {
			continue
		}
		if r.location != "" {
			fmt.Printf("[%d] /%s -> %s\n", r.status, r.path, r.location)
		} else {
			fmt.Printf("[%d] /%s\n", r.status, r.path)
		}
	}

	if ctx.Err() != nil {
		fmt.Println("\nScan interrupted by user")
	}
}
