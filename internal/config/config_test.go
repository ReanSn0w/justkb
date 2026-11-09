package config

import (
	"os"
	"testing"
)

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

func TestIconColor(t *testing.T) {
	for _, test := range []struct {
		value, want string
		invalid     bool
	}{{"", "#2563EB", false}, {"  ", "#2563EB", false}, {" #ff00aA ", "#FF00AA", false}, {"red", "", true}, {"#123", "", true}, {"#GGGGGG", "", true}} {
		t.Run(test.value, func(t *testing.T) {
			t.Setenv("JKB_ICON_COLOR", test.value)
			cfg, err := Load()
			if (err != nil) != test.invalid {
				t.Fatalf("error=%v", err)
			}
			if err == nil && cfg.IconColor != test.want {
				t.Fatalf("color=%s", cfg.IconColor)
			}
		})
	}
}
func TestUpgradeWithoutIconColor(t *testing.T) {
	t.Setenv("JKB_ICON_COLOR", "")
	if err := os.Unsetenv("JKB_ICON_COLOR"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JKB_SYSTEM_NAME", "Existing board")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.IconColor != "#2563EB" || cfg.SystemName != "Existing board" {
		t.Fatalf("upgrade config: %+v", cfg)
	}
}
