package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/ReanSn0w/justkb/internal/board"
	"github.com/ReanSn0w/justkb/internal/config"
	"github.com/ReanSn0w/justkb/internal/database"
	"golang.org/x/term"
)

const usage = `Usage:
  jkb-admin user add [options] <username>

Commands:
  user add    Create an active user

Run "jkb-admin user add --help" for command options.`

type userAddOptions struct {
	Username    string
	DisplayName string
	Email       string
	Role        string
}

func main() {
	log.SetFlags(0)
	if err := run(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(args []string) error {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help" || args[0] == "-h") {
		fmt.Fprintln(os.Stdout, usage)
		return nil
	}
	if len(args) < 2 || args[0] != "user" || args[1] != "add" {
		return errors.New(usage)
	}
	options, err := parseUserAdd(args[2:], os.Stderr)
	if errors.Is(err, flag.ErrHelp) {
		return nil
	}
	if err != nil {
		return err
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	password, err := readPassword(os.Stdin, os.Stderr)
	if err != nil {
		return err
	}
	defer clear(password)
	user, err := board.NewService(db).CreateUser(
		ctx,
		options.Username,
		options.DisplayName,
		options.Email,
		options.Role,
		string(password),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	fmt.Fprintf(os.Stdout, "created %s user %s (%s)\n", user.Role, user.Username, user.ID)
	return nil
}

func parseUserAdd(args []string, output io.Writer) (userAddOptions, error) {
	options := userAddOptions{Role: "member"}
	flags := flag.NewFlagSet("user add", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.DisplayName, "display-name", "", "name shown in the interface (defaults to username)")
	flags.StringVar(&options.Email, "email", "", "optional email address")
	flags.StringVar(&options.Role, "role", options.Role, "user role: admin or member")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: jkb-admin user add [options] <username>")
		fmt.Fprintln(output)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		return userAddOptions{}, err
	}
	if flags.NArg() != 1 {
		return userAddOptions{}, errors.New("user add requires exactly one username")
	}
	options.Username = strings.TrimSpace(flags.Arg(0))
	options.DisplayName = strings.TrimSpace(options.DisplayName)
	options.Email = strings.TrimSpace(options.Email)
	options.Role = strings.TrimSpace(options.Role)
	if options.Username == "" {
		return userAddOptions{}, errors.New("username must not be blank")
	}
	if options.DisplayName == "" {
		options.DisplayName = options.Username
	}
	if options.Role != "admin" && options.Role != "member" {
		return userAddOptions{}, errors.New("role must be admin or member")
	}
	return options, nil
}

func readPassword(input *os.File, output io.Writer) ([]byte, error) {
	inputFD := int(input.Fd())
	if !term.IsTerminal(inputFD) {
		return nil, errors.New("password input requires an interactive terminal")
	}
	fmt.Fprint(output, "Password: ")
	password, err := term.ReadPassword(inputFD)
	fmt.Fprintln(output)
	if err != nil {
		return nil, fmt.Errorf("read password: %w", err)
	}
	fmt.Fprint(output, "Confirm password: ")
	confirmation, err := term.ReadPassword(inputFD)
	fmt.Fprintln(output)
	if err != nil {
		clear(password)
		return nil, fmt.Errorf("read password confirmation: %w", err)
	}
	defer clear(confirmation)
	if !bytes.Equal(password, confirmation) {
		clear(password)
		return nil, errors.New("passwords do not match")
	}
	return password, nil
}
