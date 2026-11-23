package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"prislage/internal/fetch"
	"prislage/internal/urlx"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "parse":
		if err := runParse(os.Args[2:]); err != nil {
			if code := fetch.CodeOf(err); code != "" {
				fmt.Fprintf(os.Stderr, "error [%s]: %v\n", code, err)
			} else {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			}
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `prislage — prisbevakning CLI

Usage:
  prislage parse <url>                 Hämta en produktsida och skriv ut parsad data
  prislage parse -file <path> [-url]   Parsa sparad HTML (för regression / offline)

Flags for parse:
  -file string   lokal HTML-fil istället för HTTP
  -url string    kanonisk URL (används för store/currency när -file anges)
  -timeout dur   HTTP-timeout (default 30s)
  -json          skriv JSON istället för text

`)
}

func runParse(args []string) error {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	file := fs.String("file", "", "path to HTML file")
	pageURL := fs.String("url", "", "page URL (required with -file for store/currency hints)")
	timeout := fs.Duration("timeout", 30*time.Second, "HTTP timeout")
	asJSON := fs.Bool("json", false, "emit JSON")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}

	rawURL := fs.Arg(0)
	if *file == "" && rawURL == "" {
		return fmt.Errorf("provide a URL or -file")
	}

	var (
		res *fetch.Result
		err error
	)

	switch {
	case *file != "":
		body, readErr := os.ReadFile(*file)
		if readErr != nil {
			return readErr
		}
		u := *pageURL
		if u == "" {
			u = "https://example.invalid/product"
		}
		res, err = fetch.ParseHTML(body, u)
	default:
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()

		norm, nerr := urlx.Normalize(rawURL)
		if nerr != nil {
			return nerr
		}
		fmt.Fprintf(os.Stderr, "fetching %s\n", norm)

		client := fetch.NewHTTPClient("")
		client.Client.Timeout = *timeout
		fetcher := fetch.NewGenericFetcher(client)
		res, err = fetcher.Fetch(ctx, norm)
	}
	if err != nil {
		return err
	}

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}

	printResult(res, rawURL)
	return nil
}

func printResult(res *fetch.Result, rawURL string) {
	fmt.Printf("name:     %s\n", res.Name)
	fmt.Printf("price:    %s %s\n", res.Price.StringFixed(2), res.Currency)
	fmt.Printf("source:   %s\n", res.Source)
	fmt.Printf("store:    %s\n", res.Store)
	if res.Brand != "" {
		fmt.Printf("brand:    %s\n", res.Brand)
	}
	if res.GTIN != "" {
		fmt.Printf("gtin:     %s\n", res.GTIN)
	}
	if res.InStock != nil {
		fmt.Printf("in_stock: %v\n", *res.InStock)
	} else {
		fmt.Printf("in_stock: unknown\n")
	}
	if rawURL != "" {
		if norm, err := urlx.Normalize(rawURL); err == nil {
			fmt.Printf("normalized_url: %s\n", norm)
		}
	}
}
