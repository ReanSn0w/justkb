package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ReanSn0w/justkb/internal/auth"
	"github.com/ReanSn0w/justkb/internal/config"
	"github.com/ReanSn0w/justkb/internal/database"
	"golang.org/x/term"
)

func main() {
	if len(os.Args) != 3 {
		log.Fatal("usage: jkb-admin <username> <display-name>")
	}
	stdinFD := int(os.Stdin.Fd())
	if !term.IsTerminal(stdinFD) {
		log.Fatal("password input requires an interactive terminal")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	password, err := term.ReadPassword(stdinFD)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatal("read password: ", err)
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	user, err := auth.NewService(db).CreateAdmin(ctx, os.Args[1], os.Args[2], string(password))
	for index := range password {
		password[index] = 0
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stdout, "created admin %s (%s)\n", user.Username, user.ID)
}
