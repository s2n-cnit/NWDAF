package configuration

import (
	"github.com/hashicorp/go-hclog"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"os"
	"strconv"
	"strings"
)

var (
	MongoUsername  string
	MongoPassword  string
	MongoURI       string
	DatabaseName   string
	Collection     string
	Port           string
	BindIP         string
	CertFile       string
	KeyFile        string
	TrustedProxies []string
	LogLevel       hclog.Level
	PrometheusPort uint16
	RedisURI       string
	RedisPassword  string
	NrfURI         string
)

// LoadConfig loads environment variables and stores them in package-level variables
func LoadConfig() {
	// Load .env file if using godotenv
	err := godotenv.Load()
	if err != nil {
		logrus.Println(".env file not found, environment variables should be set directly.")
	}

	// Load environment variables into package-level variables
	MongoUsername = GetEnv("MONGO_USERNAME", "")
	MongoPassword = GetEnv("MONGO_PASSWORD", "")
	MongoURI = GetEnv("MONGO_URI", "mongodb://localhost:27017")
	DatabaseName = GetEnv("DATABASE_NAME", "nwdaf")
	Collection = GetEnv("COLLECTION_NAME", "undefined")
	Port = GetEnv("PORT", "8080")
	BindIP = GetEnv("BIND_IP", "0.0.0.0")
	CertFile = GetEnv("TLS_CERT_FILE", "certs/server.crt")
	KeyFile = GetEnv("TLS_KEY_FILE", "certs/server.key")
	TrustedProxies = strings.Split(strings.ReplaceAll(GetEnv("TRUSTED_PROXIES", "192.168.0.0/16, 127.0.0.1/32"), " ", ""), ",")
	// LOG_LEVEL is a string, so we need to convert it to uint32
	value, err := strconv.ParseUint(GetEnv("LOG_LEVEL", "1"), 10, 32) // Trace Level
	if err != nil {
		logrus.Fatalf("Error parsing string to uint32: %v", err)
	}
	LogLevel = hclog.Level(uint32(value))
	// PROMETHEUS_PORT is a string, so we need to convert it to uint16
	value, err = strconv.ParseUint(GetEnv("PROMETHEUS_PORT", "2112"), 10, 16)
	if err != nil {
		logrus.Fatalf("Error parsing string to uint16: %v", err)
	}
	PrometheusPort = uint16(value)
	// REDIS USRI
	RedisURI = GetEnv("REDIS_URI", "localhost:6379")
	NrfURI = GetEnv("NRF_URI", "localhost:29510")
	RedisPassword = GetEnv("REDIS_PASSWORD", "")
}

// GetEnv reads an environment variable or returns a default value if not set
func GetEnv(key, defaultValue string) string {
	value, exists := os.LookupEnv(key)
	if !exists {
		return defaultValue
	}
	return value
}
