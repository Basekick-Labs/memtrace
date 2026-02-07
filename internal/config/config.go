package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all configuration for memtrace
type Config struct {
	Server ServerConfig
	Log    LogConfig
	Arc    ArcConfig
	Auth   AuthConfig
	Dedup  DedupConfig
}

type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     int
	WriteTimeout    int
	ShutdownTimeout int
}

type LogConfig struct {
	Level  string
	Format string
}

type ArcConfig struct {
	URL                  string
	APIKey               string
	Database             string
	Measurement          string
	ConnectTimeout       int
	QueryTimeout         int
	WriteBatchSize       int
	WriteFlushIntervalMS int
}

type AuthConfig struct {
	Enabled bool
	DBPath  string
}

type DedupConfig struct {
	Enabled     bool
	WindowHours int
}

// Load loads configuration from file and environment
func Load() (*Config, error) {
	v := viper.New()

	setDefaults(v)

	// Environment variables
	v.SetEnvPrefix("MEMTRACE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file
	v.SetConfigName("memtrace")
	v.SetConfigType("toml")
	v.AddConfigPath(".")
	v.AddConfigPath("/etc/memtrace/")
	v.AddConfigPath("$HOME/.memtrace/")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config: %w", err)
		}
	}

	cfg := &Config{
		Server: ServerConfig{
			Host:            v.GetString("server.host"),
			Port:            v.GetInt("server.port"),
			ReadTimeout:     v.GetInt("server.read_timeout"),
			WriteTimeout:    v.GetInt("server.write_timeout"),
			ShutdownTimeout: v.GetInt("server.shutdown_timeout"),
		},
		Log: LogConfig{
			Level:  v.GetString("log.level"),
			Format: v.GetString("log.format"),
		},
		Arc: ArcConfig{
			URL:                  v.GetString("arc.url"),
			APIKey:               v.GetString("arc.api_key"),
			Database:             v.GetString("arc.database"),
			Measurement:          v.GetString("arc.measurement"),
			ConnectTimeout:       v.GetInt("arc.connect_timeout"),
			QueryTimeout:         v.GetInt("arc.query_timeout"),
			WriteBatchSize:       v.GetInt("arc.write_batch_size"),
			WriteFlushIntervalMS: v.GetInt("arc.write_flush_interval_ms"),
		},
		Auth: AuthConfig{
			Enabled: v.GetBool("auth.enabled"),
			DBPath:  v.GetString("auth.db_path"),
		},
		Dedup: DedupConfig{
			Enabled:     v.GetBool("dedup.enabled"),
			WindowHours: v.GetInt("dedup.window_hours"),
		},
	}

	return cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 9100)
	v.SetDefault("server.read_timeout", 30)
	v.SetDefault("server.write_timeout", 30)
	v.SetDefault("server.shutdown_timeout", 30)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "console")

	v.SetDefault("arc.url", "http://localhost:8000")
	v.SetDefault("arc.api_key", "")
	v.SetDefault("arc.database", "memory")
	v.SetDefault("arc.measurement", "events")
	v.SetDefault("arc.connect_timeout", 5)
	v.SetDefault("arc.query_timeout", 30)
	v.SetDefault("arc.write_batch_size", 100)
	v.SetDefault("arc.write_flush_interval_ms", 1000)

	v.SetDefault("auth.enabled", true)
	v.SetDefault("auth.db_path", "./data/memtrace.db")

	v.SetDefault("dedup.enabled", true)
	v.SetDefault("dedup.window_hours", 24)
}
