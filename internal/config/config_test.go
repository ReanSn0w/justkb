package config

import "testing"

func TestSystemName(t *testing.T) {
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
	t.Setenv("JKB_SYSTEM_NAME", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SystemName != "JKB" {
		t.Fatalf("SystemName=%q", cfg.SystemName)
	}
}

func TestSQLitePathsDefaultInsideStorage(t *testing.T) {
	t.Setenv("JKB_DATABASE_PATH", "")
	t.Setenv("JKB_STORAGE_PATH", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DatabasePath != "./storage/jkb.sqlite" {
		t.Fatalf("DatabasePath=%q", cfg.DatabasePath)
	}
	if cfg.StoragePath != "./storage/files" {
		t.Fatalf("StoragePath=%q", cfg.StoragePath)
	}
}
