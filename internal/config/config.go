package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Config holds all application configuration
type Config struct {
	Port          string // Panel HTTP port
	DBPath        string // SQLite database path
	JWTSecret     string // JWT signing secret
	CaddyBin      string // Path to caddy binary
	CaddyfilePath string // Path to generated Caddyfile
	LogDir        string // Directory for Caddy logs
	DataDir       string // Data directory root
	AdminAPI      string // Caddy admin API URL
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	dataDir := envOrDefault("SERVERDASH_DATA_DIR", "./data")

	// Ensure directories exist early so we can write the secret file
	os.MkdirAll(dataDir, 0755)

	cfg := &Config{
		Port:          envOrDefault("SERVERDASH_PORT", "39921"),
		DBPath:        envOrDefault("SERVERDASH_DB_PATH", filepath.Join(dataDir, "serverdash.db")),
		JWTSecret:     resolveJWTSecret(dataDir),
		CaddyBin:      envOrDefault("SERVERDASH_CADDY_BIN", "caddy"),
		CaddyfilePath: envOrDefault("SERVERDASH_CADDYFILE_PATH", filepath.Join(dataDir, "Caddyfile")),
		LogDir:        envOrDefault("SERVERDASH_LOG_DIR", filepath.Join(dataDir, "logs")),
		DataDir:       dataDir,
		AdminAPI:      envOrDefault("SERVERDASH_ADMIN_API", "http://localhost:2019"),
	}

	// Ensure directories exist
	os.MkdirAll(cfg.LogDir, 0755)
	os.MkdirAll(filepath.Join(dataDir, "backups"), 0755)

	return cfg
}

// resolveJWTSecret determines the JWT secret using this priority:
//  1. SERVERDASH_JWT_SECRET env var (if set and not an insecure default)
//  2. Persisted secret in data/.jwt_secret
//  3. Auto-generate a new cryptographic random secret and persist it
func resolveJWTSecret(dataDir string) string {
	// Known insecure defaults that must be rejected.
	insecureDefaults := map[string]bool{
		"serverdash-change-me-in-production": true,
		"change-me-in-production":         true,
	}

	// 1. Explicit env var takes precedence (if not an insecure default)
	if envSecret := envOrDefault("SERVERDASH_JWT_SECRET", ""); envSecret != "" && !insecureDefaults[envSecret] {
		return envSecret
	}

	// 2. Try to load persisted secret
	secretFile := filepath.Join(dataDir, ".jwt_secret")
	if data, err := os.ReadFile(secretFile); err == nil {
		secret := strings.TrimSpace(string(data))
		if secret != "" {
			return secret
		}
	}

	// 3. Generate a cryptographically random secret and persist it
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
		log.Fatalf("FATAL: failed to generate JWT secret: %v", err)
	}
	secret := hex.EncodeToString(secretBytes)

	if err := os.WriteFile(secretFile, []byte(secret+"\n"), 0600); err != nil {
		log.Printf("⚠️  Could not persist JWT secret to %s: %v", secretFile, err)
		log.Printf("   Set SERVERDASH_JWT_SECRET env var to ensure stable sessions across restarts.")
	} else {
		log.Printf("🔑 Generated new JWT secret and saved to %s", secretFile)
	}

	return secret
}

func envOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func envOrLegacyDefault(key, legacyKey, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	if val := os.Getenv(legacyKey); val != "" {
		return val
	}
	return defaultVal
}
