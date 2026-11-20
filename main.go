package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {

	// Definir los flags, es decir, "parámetros" de la ejecución
	target := flag.String("u", "", "Target URL")
	wordlist := flag.String("w", "", "Dictionary path, ej: test.txt")
	flag.Parse()

	if *target == "" || *wordlist == "" {
		fmt.Println("Error: You must indicate -u and -w")
		flag.Usage()
		os.Exit(1)
	}

	fmt.Println("Target:", *target)
	fmt.Println("Dictionary:", *wordlist)

	file, err := os.Open(*wordlist)
	if err != nil {
		fmt.Println("Error: Can't open the specified dictionary:", err)
		os.Exit(1)
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Quitamos / para evitar mal formato
	base := strings.TrimRight(*target, "/")

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Igual forma evitar mal formato y asegurarnos de solo un "/"
		path := strings.TrimLeft(line, "/")
		fullURL := base + "/" + path

		fmt.Println("URL:", fullURL)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("An error ocurred reading the dictionary:", err)
		os.Exit(1)
	}

}
