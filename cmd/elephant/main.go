package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"elephant/internal/api"
	"elephant/internal/db"
	"elephant/internal/ingest"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 && len(os.Args[1]) > 0 && os.Args[1][0] != '-' {
		cmd = os.Args[1]
	}
	store, err := openStore()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	switch cmd {
	case "serve":
		if err := serve(store); err != nil {
			log.Fatal(err)
		}
	case "seed":
		if err := ingest.Seed(context.Background(), store); err != nil {
			log.Fatal(err)
		}
		fmt.Println("seeded")
	case "sync":
		result, err := ingest.New(store).Sync(context.Background(), "")
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("synced %d unchanged %d failed %d added %d\n", result.Synced, result.NotModified, result.Failed, result.ItemsAdded)
		for _, item := range result.Errors {
			fmt.Println(item)
		}
	case "import":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: elephant import <file> [youtube|opml]")
			os.Exit(2)
		}
		format := ""
		if len(os.Args) > 3 {
			format = os.Args[3]
		}
		body, err := os.ReadFile(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		result, err := ingest.ImportBytes(context.Background(), store, body, ingest.ImportOptions{Filename: os.Args[2], Format: format})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("added %d skipped %d total %d\n", result.Added, result.Skipped, result.Total)
	default:
		fmt.Fprintln(os.Stderr, "usage: elephant [serve|seed|sync|import]")
		os.Exit(2)
	}
}

func openStore() (*db.Store, error) {
	path := env("DATABASE_PATH", "data/elephant.db")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	zone := env("ELEPHANT_TIMEZONE", "America/Los_Angeles")
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return nil, fmt.Errorf("ELEPHANT_TIMEZONE: %w", err)
	}
	return db.Open(path, loc)
}

func serve(store *db.Store) error {
	port := env("PORT", "8080")
	if port[0] != ':' {
		port = ":" + port
	}
	handler := api.WithStatic(api.New(store, os.Getenv("ELEPHANT_API_TOKEN")), env("STATIC_DIR", "dist"))
	server := &http.Server{Addr: port, Handler: handler}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}()
	log.Printf("elephant listening on %s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
