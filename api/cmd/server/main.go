package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"prislage/internal/analyze"
	"prislage/internal/config"
	"prislage/internal/db"
	"prislage/internal/fetch"
	"prislage/internal/handlers"
	"prislage/internal/notify"
	"prislage/internal/service"
	"prislage/internal/store"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	items := store.NewItems(pool)
	stores := store.NewStores(pool)
	analyses := store.NewAnalyses(pool)
	client := fetch.NewHTTPClient(cfg.UserAgent)
	fetcher := fetch.NewGenericFetcher(client)
	push := &notify.WebPush{
		Items: items, PublicKey: cfg.VAPIDPublicKey, PrivateKey: cfg.VAPIDPrivateKey,
		Subject: cfg.VAPIDSubject, Log: log,
	}
	svc := service.New(items, stores, fetcher, push, log)
	svc.Analyses = analyses
	if cfg.AnthropicAPIKey != "" {
		svc.Claude = analyze.NewClient(cfg.AnthropicAPIKey, cfg.AnthropicModel)
		log.Info("analyze enabled", "model", cfg.AnthropicModel)
	} else {
		log.Warn("ANTHROPIC_API_KEY saknas — POST /api/analyze inaktiverad")
	}

	sched := &fetch.Scheduler{
		Fetcher: svc, Interval: cfg.FetchInterval, RatePerMinute: cfg.FetchRatePerMinute, Log: log,
	}
	sched.Start(ctx)

	api := &handlers.API{Svc: svc, Cfg: cfg, Stores: stores}
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      150 * time.Second,
	}

	go func() {
		log.Info("api listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("listen", "err", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
	defer c()
	_ = srv.Shutdown(shutdownCtx)
}
