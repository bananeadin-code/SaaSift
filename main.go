package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type result struct {
	path   string
	status int
	err    error
}

func checkURL(client *http.Client, fullURL string) (int, error) {
	resp, err := client.Get(fullURL)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func worker(client *http.Client, base string, paths <-chan string,
	results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()

	for path := range paths {
		status, err := checkURL(client, base+"/"+path)
		results <- result{path: path, status: status, err: err}
	}
}

func main() {

	// Definir los flags, es decir, "parámetros" de la ejecución
	target := flag.String("u", "", "Target URL")
	wordlist := flag.String("w", "", "Dictionary path, ej: test.txt")
	threads := flag.Int("t", 10, "Number of concurrent workers")
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

	file, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println("Error: Can't open the specified dictionary:", err)
		os.Exit(1)
	}

	defer file.Close()

	// Quitamos / para evitar mal formato
	base := strings.TrimRight(*target, "/")

	client := &http.Client{Timeout: 10 * time.Second}

	paths := make(chan string)
	results := make(chan result)

	var wg sync.WaitGroup

	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go worker(client, base, paths, results, &wg)
	}

	go func() {
		defer close(paths)

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			paths <- strings.TrimLeft(line, "/")
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
			fmt.Println("[ERR]", r.path, r.err)
			continue
		}
		if r.status != http.StatusNotFound {
			fmt.Printf("[%d] /%s\n", r.status, r.path)
		}
	}
}
