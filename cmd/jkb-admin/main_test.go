package main

import (
	"io"
	"testing"
)

func TestParseUserAddDefaults(t *testing.T) {
	options, err := parseUserAdd([]string{"ada"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if options.Username != "ada" || options.DisplayName != "ada" || options.Role != "member" || options.Email != "" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestParseUserAddOptions(t *testing.T) {
	options, err := parseUserAdd([]string{
		"--display-name", "Ada Lovelace",
		"--email", "ada@example.test",
		"--role", "admin",
		"ada",
	}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if options.Username != "ada" || options.DisplayName != "Ada Lovelace" || options.Role != "admin" || options.Email != "ada@example.test" {
		t.Fatalf("unexpected options: %#v", options)
	}
}

func TestParseUserAddRejectsInvalidInput(t *testing.T) {
	for name, args := range map[string][]string{
		"missing username": nil,
		"extra username":   {"ada", "grace"},
		"invalid role":     {"--role", "owner", "ada"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseUserAdd(args, io.Discard); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
