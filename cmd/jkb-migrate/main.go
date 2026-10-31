package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ReanSn0w/justkb/internal/config"
	"github.com/ReanSn0w/justkb/internal/database"
	"github.com/pressly/goose/v3"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: jkb-migrate <up|down|status>")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		log.Fatal(err)
	}

	var runErr error
	switch os.Args[1] {
	case "up":
		runErr = goose.Up(db, "migrations")
	case "down":
		runErr = goose.Down(db, "migrations")
	case "status":
		runErr = goose.Status(db, "migrations")
	default:
		runErr = fmt.Errorf("unknown migration command %q", os.Args[1])
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
}
