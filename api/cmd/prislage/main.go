package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"prislage/internal/config"
	"prislage/internal/db"
	"prislage/internal/fetch"
	"prislage/internal/service"
	"prislage/internal/store"
	"prislage/internal/urlx"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "parse":
		exitOn(runParse(os.Args[2:]))
	case "fetch":
		exitOn(runFetch(os.Args[2:]))
	case "migrate":
		exitOn(runMigrate(os.Args[2:]))
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func exitOn(err error) {
	if err == nil {
		return
	}
	if code := fetch.CodeOf(err); code != "" {
		fmt.Fprintf(os.Stderr, "error [%s]: %v\n", code, err)
	} else {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
	os.Exit(1)
}

func usage() {
	fmt.Fprintf(os.Stderr, `prislage — prisbevakning CLI

Usage:
  prislage parse <url>                 Hämta/parsa en produktsida
  prislage parse -file <path> [-url]   Parsa sparad HTML
  prislage fetch                       Hämta priser för alla auto-butiker (CronJob)
  prislage migrate                     Kör goose-migrationer

`)
}

func runParse(args []string) error {
	fs := flag.NewFlagSet("parse", flag.ContinueOnError)
	file := fs.String("file", "", "path to HTML file")
	pageURL := fs.String("url", "", "page URL")
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
		res, err = fetch.NewGenericFetcher(client).Fetch(ctx, norm)
	}
	if err != nil {
		return err
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(res)
	}
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
	if res.SKU != "" {
		fmt.Printf("sku:      %s\n", res.SKU)
	}
	if res.InStock != nil {
		fmt.Printf("in_stock: %v\n", *res.InStock)
	}
	return nil
}

func runFetch(args []string) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	items := store.NewItems(pool)
	stores := store.NewStores(pool)
	fetcher := fetch.NewGenericFetcher(fetch.NewHTTPClient(cfg.UserAgent))
	svc := service.New(items, stores, fetcher, nil, log)
	ok, fail := svc.FetchAllAuto(ctx, cfg.FetchRatePerMinute)
	log.Info("fetch finished", "ok", ok, "fail", fail)
	if fail > 0 && ok == 0 {
		return fmt.Errorf("all %d fetches failed", fail)
	}
	return nil
}

func runMigrate(args []string) error {
	fs := flag.NewFlagSet("migrate", flag.ContinueOnError)
	dir := fs.String("dir", "", "migrations directory")
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	migDir := *dir
	if migDir == "" {
		migDir = cfg.MigrationsDir
	}
	sqlDB, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.Up(sqlDB, migDir)
}
