package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

const defaultDataDir = "data"

// Database selects the persistence driver and its driver-specific DSN.
type Database struct {
	Driver string `json:"driver"`
	DSN    string `json:"dsn"`
}

// Config is the validated runtime configuration consumed by the application.
type Config struct {
	Address           string
	SiteURL           string
	UploadDirectFirst bool
	CORSAllowOrigin   string
	Database          Database
	DataDir           string
	WWWRoot           string
	RealIPHeader      string
	MaxTextSize       int64
	MaxLinkLength     int
	MaxUploadFileSize int64
	UploadChunkSize   int64
	UploadSessionTTL  time.Duration
	UploadWorkers     int
}

type fileConfig struct {
	Address           string   `json:"address"`
	SiteURL           string   `json:"site_url"`
	UploadDirectFirst bool     `json:"upload_direct_first"`
	CORSAllowOrigin   string   `json:"cors_allow_origin"`
	Database          Database `json:"database"`
	DataDir           string   `json:"data_dir"`
	WWWRoot           string   `json:"www_root"`
	RealIPHeader      string   `json:"real_ip_header"`
	MaxTextSize       int64    `json:"max_text_size"`
	MaxLinkLength     int      `json:"max_link_length"`
	MaxUploadFileSize int64    `json:"max_upload_file_size"`
	UploadChunkSize   int64    `json:"upload_chunk_size"`
	UploadSessionTTL  string   `json:"upload_session_ttl"`
	UploadWorkers     int      `json:"upload_workers"`
}

// Load reads the configured JSON file, applies environment overrides, and
// returns a validated configuration.
func Load() (Config, error) {
	_ = godotenv.Load()
	dataDir := env("DATA_DIR", defaultDataDir)
	path := env("CONFIG_FILE", filepath.Join(dataDir, "config.json"))
	cfg, err := LoadFile(path)
	if err != nil {
		return Config{}, err
	}
	if err := applyEnvironment(&cfg); err != nil {
		return Config{}, err
	}
	return validate(cfg)
}

// LoadFile creates a default configuration when path does not exist, then
// parses it. Environment overrides are applied only by Load.
func LoadFile(path string) (Config, error) {
	if err := ensureConfigFile(path); err != nil {
		return Config{}, err
	}
	payload, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %s: %w", path, err)
	}
	settings := defaultFileConfig()
	if err := json.Unmarshal(payload, &settings); err != nil {
		return Config{}, fmt.Errorf("parse config %s: %w", path, err)
	}
	ttl, err := time.ParseDuration(settings.UploadSessionTTL)
	if err != nil {
		return Config{}, fmt.Errorf("parse upload_session_ttl: %w", err)
	}
	cfg := Config{
		Address: settings.Address, SiteURL: settings.SiteURL, UploadDirectFirst: settings.UploadDirectFirst,
		CORSAllowOrigin: settings.CORSAllowOrigin, Database: settings.Database, DataDir: settings.DataDir,
		WWWRoot: settings.WWWRoot, RealIPHeader: settings.RealIPHeader,
		MaxTextSize: settings.MaxTextSize, MaxLinkLength: settings.MaxLinkLength,
		MaxUploadFileSize: settings.MaxUploadFileSize, UploadChunkSize: settings.UploadChunkSize,
		UploadSessionTTL: ttl, UploadWorkers: settings.UploadWorkers,
	}
	return validate(cfg)
}

func defaultFileConfig() fileConfig {
	return fileConfig{
		Address:           ":5328",
		UploadDirectFirst: true,
		CORSAllowOrigin:   "*",
		Database:          Database{Driver: "sqlite", DSN: filepath.Join(defaultDataDir, "clipbox.db")},
		DataDir:           defaultDataDir,
		WWWRoot:           "www",
		RealIPHeader:      "X-Real-IP",
		MaxTextSize:       5 * 1024 * 1024,
		MaxLinkLength:     2048,
		MaxUploadFileSize: 500 * 1024 * 1024,
		UploadChunkSize:   4 * 1024 * 1024,
		UploadSessionTTL:  "10m",
		UploadWorkers:     4,
	}
}

func ensureConfigFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect config %s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	payload, err := json.MarshalIndent(defaultFileConfig(), "", "  ")
	if err != nil {
		return fmt.Errorf("encode default config: %w", err)
	}
	payload = append(payload, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil
		}
		return fmt.Errorf("create config %s: %w", path, err)
	}
	if _, err := file.Write(payload); err != nil {
		_ = file.Close()
		return fmt.Errorf("write config %s: %w", path, err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close config %s: %w", path, err)
	}
	return nil
}

func applyEnvironment(cfg *Config) error {
	setString("ADDRESS", &cfg.Address)
	setString("DATA_DIR", &cfg.DataDir)
	setString("WWW_ROOT", &cfg.WWWRoot)
	setString("REAL_IP_HEADER", &cfg.RealIPHeader)
	databaseDriver, driverSet := lookup("DATABASE_DRIVER")
	if driverSet {
		cfg.Database.Driver = databaseDriver
	}

	if dsn, ok := lookup("DATABASE_DSN"); ok {
		if !driverSet {
			cfg.Database.Driver = "mysql"
		}
		cfg.Database.DSN = dsn
	} else if rawURL, ok := lookup("DATABASE_URL"); ok {
		dsn, err := sqlalchemyURLToDSN(rawURL)
		if err != nil {
			return err
		}
		cfg.Database.Driver = "mysql"
		cfg.Database.DSN = dsn
	}

	if err := setInt64("MAX_TEXT_SIZE", &cfg.MaxTextSize); err != nil {
		return err
	}
	if err := setInt("MAX_LINK_LENGTH", &cfg.MaxLinkLength); err != nil {
		return err
	}
	if err := setInt64("MAX_UPLOAD_FILE_SIZE", &cfg.MaxUploadFileSize); err != nil {
		return err
	}
	if err := setInt64("UPLOAD_CHUNK_SIZE", &cfg.UploadChunkSize); err != nil {
		return err
	}
	if err := setInt("UPLOAD_WORKERS", &cfg.UploadWorkers); err != nil {
		return err
	}
	if value, ok := lookup("UPLOAD_SESSION_TTL"); ok {
		duration, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("parse UPLOAD_SESSION_TTL: %w", err)
		}
		cfg.UploadSessionTTL = duration
	}
	return nil
}

func validate(cfg Config) (Config, error) {
	cfg.SiteURL = strings.TrimRight(strings.TrimSpace(cfg.SiteURL), "/")
	if cfg.SiteURL != "" {
		parsed, err := url.ParseRequestURI(cfg.SiteURL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Path != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil {
			return Config{}, fmt.Errorf("site_url must be an absolute HTTP(S) URL")
		}
	}
	cfg.CORSAllowOrigin = strings.TrimSpace(cfg.CORSAllowOrigin)
	if cfg.CORSAllowOrigin == "" {
		cfg.CORSAllowOrigin = "*"
	}
	if strings.Contains(cfg.CORSAllowOrigin, ",") {
		return Config{}, fmt.Errorf("cors_allow_origin must be * or one origin")
	}
	if cfg.CORSAllowOrigin != "*" {
		origin, err := url.ParseRequestURI(cfg.CORSAllowOrigin)
		if err != nil || origin.Host == "" || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.User != nil {
			return Config{}, fmt.Errorf("cors_allow_origin must be * or an absolute HTTP(S) origin")
		}
	}
	cfg.Database.Driver = strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	cfg.Database.DSN = strings.TrimSpace(cfg.Database.DSN)
	switch cfg.Database.Driver {
	case "sqlite", "sqlite3":
		cfg.Database.Driver = "sqlite"
	case "mysql":
		if !strings.Contains(cfg.Database.DSN, "parseTime=") {
			separator := "?"
			if strings.Contains(cfg.Database.DSN, "?") {
				separator = "&"
			}
			cfg.Database.DSN += separator + "parseTime=true&loc=UTC"
		}
	default:
		return Config{}, fmt.Errorf("unsupported database driver %q", cfg.Database.Driver)
	}
	if cfg.Database.DSN == "" {
		return Config{}, fmt.Errorf("database DSN must not be empty")
	}
	if strings.TrimSpace(cfg.Address) == "" {
		return Config{}, fmt.Errorf("address must not be empty")
	}
	if cfg.MaxTextSize <= 0 || cfg.MaxLinkLength <= 0 || cfg.UploadChunkSize <= 0 || cfg.MaxUploadFileSize <= 0 || cfg.UploadWorkers <= 0 {
		return Config{}, fmt.Errorf("size, length, and worker settings must be positive")
	}
	if cfg.UploadSessionTTL <= 0 {
		return Config{}, fmt.Errorf("upload_session_ttl must be positive")
	}
	cfg.DataDir = filepath.Clean(cfg.DataDir)
	cfg.WWWRoot = filepath.Clean(cfg.WWWRoot)
	return cfg, nil
}

func env(key, fallback string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	return fallback
}

func lookup(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	value = strings.TrimSpace(value)
	return value, ok && value != ""
}

func setString(key string, target *string) {
	if value, ok := lookup(key); ok {
		*target = value
	}
}

func setInt64(key string, target *int64) error {
	value, ok := lookup(key)
	if !ok {
		return nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fmt.Errorf("parse %s: %w", key, err)
	}
	*target = parsed
	return nil
}

func setInt(key string, target *int) error {
	var parsed int64
	if err := setInt64(key, &parsed); err != nil {
		return err
	}
	if _, ok := lookup(key); ok {
		*target = int(parsed)
	}
	return nil
}

func sqlalchemyURLToDSN(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	if parsed.Scheme != "mysql" && parsed.Scheme != "mysql+pymysql" {
		return "", fmt.Errorf("unsupported database scheme %q", parsed.Scheme)
	}
	password, _ := parsed.User.Password()
	credentials := parsed.User.Username()
	if password != "" {
		credentials += ":" + password
	}
	host := parsed.Host
	if !strings.Contains(host, ":") {
		host += ":3306"
	}
	database := strings.TrimPrefix(parsed.Path, "/")
	if database == "" {
		return "", fmt.Errorf("DATABASE_URL has no database name")
	}
	query := parsed.Query()
	return fmt.Sprintf("%s@tcp(%s)/%s?%s", credentials, host, database, query.Encode()), nil
}
