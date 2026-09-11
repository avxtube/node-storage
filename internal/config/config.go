package config

import (
	"os"

	"github.com/joho/godotenv"
)

// AppConfig holds the application configuration loaded from environment variables.
var AppConfig Config

// Config represents the application configuration.
type Config struct {
	Host     string
	Port     string
	MongoURI string

	StorageId            string
	StorageEncryptionKey string
}

// Load reads configuration from environment variables (and .env file).
func Load() {
	// Load .env file if present (ignore error if not found)
	godotenv.Load()

	AppConfig = Config{
		// content-node runs on a different host and fetches sprite/static assets
		// from this server. Bind all interfaces by default; production access
		// should be restricted to trusted source IPs at the firewall layer.
		Host:                 getEnv("HOST", "0.0.0.0"),
		Port:                 getEnv("PORT", "8888"),
		MongoURI:             getEnv("DATABASE_URL", "mongodb://localhost:27017"),
		StorageId:            getEnv("STORAGE_ID", ""),
		StorageEncryptionKey: getEnv("STORAGE_ENCRYPTION_KEY", getEnv("BETTER_AUTH_SECRET", "")),
	}
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
