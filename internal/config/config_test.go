package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadFileCreatesSQLiteConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "config.json")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != filepath.Join("data", "clipbox.db") {
		t.Fatalf("database = %#v, want default SQLite", cfg.Database)
	}
	if cfg.UploadSessionTTL != 10*time.Minute {
		t.Fatalf("upload TTL = %s", cfg.UploadSessionTTL)
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), `"driver": "sqlite"`) {
		t.Fatalf("generated configuration does not contain SQLite: %s", payload)
	}
}

func TestLoadFileAcceptsMySQLConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{
		"database": {"driver": "mysql", "dsn": "user:pass@tcp(db:3306)/clipbox?charset=utf8mb4"},
		"upload_session_ttl": "20m"
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Driver != "mysql" || !strings.Contains(cfg.Database.DSN, "parseTime=true&loc=UTC") {
		t.Fatalf("database = %#v", cfg.Database)
	}
	if cfg.UploadSessionTTL != 20*time.Minute {
		t.Fatalf("upload TTL = %s", cfg.UploadSessionTTL)
	}
}

func TestEnvironmentDatabaseDSNSelectsMySQL(t *testing.T) {
	t.Setenv("DATABASE_DSN", "root@tcp(localhost:3306)/clipbox")
	cfg := Config{Database: Database{Driver: "sqlite", DSN: "data/clipbox.db"}}
	if err := applyEnvironment(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Driver != "mysql" || cfg.Database.DSN != "root@tcp(localhost:3306)/clipbox" {
		t.Fatalf("database override = %#v", cfg.Database)
	}
}

func TestEnvironmentCanOverrideSQLiteDSN(t *testing.T) {
	t.Setenv("DATABASE_DRIVER", "sqlite")
	t.Setenv("DATABASE_DSN", "storage/clipbox.db")
	cfg := Config{Database: Database{Driver: "sqlite", DSN: "data/clipbox.db"}}
	if err := applyEnvironment(&cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != "storage/clipbox.db" {
		t.Fatalf("database override = %#v", cfg.Database)
	}
}
