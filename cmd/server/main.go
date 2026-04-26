package main

import (
	"context"
	"flag"
	"log"
	"net/http"

	"github.com/jaochai/video-fb/internal/config"
	"github.com/jaochai/video-fb/internal/database"
	"github.com/jaochai/video-fb/internal/router"
)

func main() {
	migrateFlag := flag.Bool("migrate", false, "Run database migrations")
	flag.Parse()

	cfg := config.Load()
	ctx := context.Background()

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()
	log.Println("Connected to database")

	if *migrateFlag {
		if err := database.RunMigrations(ctx, pool, "migrations"); err != nil {
			log.Fatalf("Failed to run migrations: %v", err)
		}
		log.Println("Migrations complete")
		return
	}

	r := router.New(pool, cfg.APIKey)

	addr := ":" + cfg.Port
	log.Printf("Starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
