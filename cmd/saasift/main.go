package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/bananeadin-code/saasift/internal/filter"
	"github.com/bananeadin-code/saasift/internal/httpclient"
	"github.com/bananeadin-code/saasift/internal/output"
	"github.com/bananeadin-code/saasift/internal/scanner"
	"github.com/bananeadin-code/saasift/internal/wordlists"
)

func fail(args ...any) {
	fmt.Fprintln(os.Stderr, append([]any{"Error:"}, args...)...)
	os.Exit(1)
}

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
	calibrate := flag.Bool("calibrate", true, "Auto-detect soft-404 responses (disable with -calibrate=false)")
	verbose := flag.Bool("v", false, "Also show filtered results and the reason")
	filterCodes := intSet{404: true}
	flag.Var(filterCodes, "fc", "Hide these status codes, comma-separated")
	filterSizes := intSet{}
	flag.Var(filterSizes, "fs", "Hide these response sizes (bytes), comma-separated")
	filterWords := intSet{}
	flag.Var(filterWords, "fw", "Hide these word counts, comma-separated")
	outFile := flag.String("o", "", "Save results to a file: .json or .csv")
	noColor := flag.Bool("no-color", false, "Disable colored output")
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
		fail(err)
	}

	var data []byte
	switch *wordlist {
	case "":
		data = []byte(wordlists.SaaS)
	case "-":
		data, err = io.ReadAll(os.Stdin)
	default:
		data, err = os.ReadFile(*wordlist)
	}
	if err != nil {
		fail("could not read wordlist:", err)
	}

	colors := !*noColor && os.Getenv("NO_COLOR") == "" && output.IsTerminal(os.Stdout)
	reporters := output.Multi{output.NewConsole(os.Stdout, colors)}

	if *outFile != "" {
		ext := filepath.Ext(*outFile)
		if ext != ".json" && ext != ".csv" {
			fail("-o must end in .json or .csv")
		}
		f, err := os.Create(*outFile)
		if err != nil {
			fail("could not create output file:", err)
		}
		defer f.Close()

		if ext == ".json" {
			reporters = append(reporters, output.NewJSON(f, *target))
		} else {
			csvRep, err := output.NewCSV(f)
			if err != nil {
				fail(err)
			}
			reporters = append(reporters, csvRep)
		}
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
		Calibrate: *calibrate,
	}, client)

	userFilter := filter.Filter{
		Codes: filterCodes,
		Sizes: filterSizes,
		Words: filterWords,
	}

	stats := output.Stats{Total: scanner.CountPaths(data), Start: time.Now()}
	progress := output.NewProgress(os.Stderr, output.IsTerminal(os.Stderr))

	for r := range s.Run(ctx, bytes.NewReader(data)) {
		if r.Err != nil && errors.Is(r.Err, context.Canceled) {
			continue
		}
		stats.Processed++

		switch {
		case r.Err != nil:
			stats.Errors++
			progress.Clear()
			fmt.Fprintln(os.Stderr, "[ERR]", r.Path, r.Err)

		case userFilter.Match(r.Status, r.Size, r.Words),
			r.Filtered != "" && !*verbose:
			stats.Filtered++

		default:
			if r.Filtered == "" {
				stats.Found++
			} else {
				stats.Filtered++
			}
			progress.Clear()
			if err := reporters.Report(r); err != nil {
				fmt.Fprintln(os.Stderr, "Error writing results:", err)
			}
		}
		progress.Draw(stats, false)
	}

	progress.Clear()
	if err := reporters.Close(); err != nil {
		fmt.Fprintln(os.Stderr, "Error saving results:", err)
	}
	if ctx.Err() != nil {
		fmt.Fprintln(os.Stderr, "Scan interrupted by user")
	}
	fmt.Fprintln(os.Stderr, stats.Summary())
	if *outFile != "" {
		fmt.Fprintln(os.Stderr, "Results saved to", *outFile)
	}
}
