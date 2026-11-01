package config

import "testing"

func TestSystemName(t *testing.T) {
	t.Setenv("JKB_DATABASE_URL", "postgres://jkb@localhost/jkb")
	t.Setenv("JKB_SYSTEM_NAME", "  GostAssistent  ")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SystemName != "GostAssistent" {
		t.Fatalf("SystemName=%q", cfg.SystemName)
	}
}

func TestSystemNameDefaultsToJKB(t *testing.T) {
	t.Setenv("JKB_DATABASE_URL", "postgres://jkb@localhost/jkb")
	t.Setenv("JKB_SYSTEM_NAME", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SystemName != "JKB" {
		t.Fatalf("SystemName=%q", cfg.SystemName)
	}
}
