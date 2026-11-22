package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/bananeadin-code/saasift/internal/httpclient"
	"github.com/bananeadin-code/saasift/internal/scanner"
	"github.com/bananeadin-code/saasift/internal/wordlists"
)

func main() {

	target := flag.String("u", "", "Target URL, e.g. http://localhost:8000")
	wordlist := flag.String("w", "", "Wordlist path, or - for stdin (default: built-in SaaS wordlist)")
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

	if *target == "" {
		fmt.Println("Error: you must specify -u")
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

	var input io.Reader
	switch *wordlist {
	case "":
		input = strings.NewReader(wordlists.SaaS)
	case "-":
		input = os.Stdin
	default:
		file, err := os.Open(*wordlist)
		if err != nil {
			fmt.Println("Error: could not open wordlist:", err)
			os.Exit(1)
		}
		defer file.Close()
		input = file
	}

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

	for r := range s.Run(ctx, input) {
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
		line := fmt.Sprintf("[%d] /%s", r.Status, r.Path)
		if r.Location != "" {
			line += " -> " + r.Location
		}
		fmt.Printf("%-50s [size:%d words:%d lines:%d type:%s]\n",
			line, r.Size, r.Words, r.Lines, r.ContentType)
	}

	if ctx.Err() != nil {
		fmt.Println("\nScan interrupted by user")
	}
}
