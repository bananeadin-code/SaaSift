package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/bananeadin-code/saasift/internal/httpclient"
	"github.com/bananeadin-code/saasift/internal/scanner"
)

func main() {

	target := flag.String("u", "", "Target URL, e.g. http://localhost:8000")
	wordlist := flag.String("w", "", "Wordlist path, e.g. test.txt")
	threads := flag.Int("t", 10, "Number of concurrent workers")
	timeout := flag.Duration("timeout", 10*time.Second, "Timeout per request, e.g. 5s, 500ms")
	retries := flag.Int("retries", 2, "Retries on network errors or timeouts")
	userAgent := flag.String("a", scanner.DefaultUserAgent, "User-Agent header")
	rate := flag.Int("rate", 0, "Max requests per second (0 = unlimited)")
	proxy := flag.String("proxy", "", "Proxy URL, e.g. http://127.0.0.1:8080")
	insecure := flag.Bool("k", false, "Skip TLS certificate verification")
	headers := headerFlags{}
	flag.Var(headers, "H", "Custom header 'Name: value' (repeatable)")
	flag.Parse()

	if *target == "" || *wordlist == "" {
		fmt.Println("Error: you must specify -u and -w")
		flag.Usage()
		os.Exit(1)
	}
	if *threads < 1 {
		fmt.Println("Error: -t must be at least 1")
		os.Exit(1)
	}
	if *retries < 0 {
		fmt.Println("Error: -retries cannot be negative")
		os.Exit(1)
	}

	if *rate < 0 || *rate > 10000 {
		fmt.Println("Error: -rate must be between 0 and 10000")
		os.Exit(1)
	}

	client, err := httpclient.New(httpclient.Options{
		Threads:  *threads,
		Timeout:  *timeout,
		Proxy:    *proxy,
		Insecure: *insecure,
	})
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}

	file, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println("Error: could not open wordlist:", err)
		os.Exit(1)
	}
	defer file.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	s := scanner.New(scanner.Config{
		BaseURL:   *target,
		Threads:   *threads,
		Retries:   *retries,
		UserAgent: *userAgent,
		Headers:   http.Header(headers),
		Rate:      *rate,
	}, client)

	for r := range s.Run(ctx, file) {
		if r.Err != nil {
			if errors.Is(r.Err, context.Canceled) {
				continue
			}
			fmt.Println("[ERR]", r.Path, r.Err)
			continue
		}
		if r.Status == http.StatusNotFound {
			continue
		}
		if r.Location != "" {
			fmt.Printf("[%d] /%s -> %s\n", r.Status, r.Path, r.Location)
		} else {
			fmt.Printf("[%d] /%s\n", r.Status, r.Path)
		}
	}

	if ctx.Err() != nil {
		fmt.Println("\nScan interrupted by user")
	}
}
