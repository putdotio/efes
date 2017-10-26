package config

import "github.com/BurntSushi/toml"

// TrackerConfig holds configuration values for Tracker.
type TrackerConfig struct {
	Debug         bool   `toml:"debug"`
	ListenAddress string `toml:"listen_address"`
}

// DatabaseConfig holds configuration values for database.
type DatabaseConfig struct {
	DSN string `toml:"dsn"`
}

// ServerConfig holds configuration values for Server.
type ServerConfig struct {
	Debug bool `toml:"debug"`
}

// Config holds configuration values for all Efes components.
type Config struct {
	Tracker  TrackerConfig
	Server   ServerConfig
	Database DatabaseConfig
}

// New parses a TOML file and returns new Config.
func New(configFile string) (*Config, error) {
	var c Config
	_, err := toml.DecodeFile(configFile, &c)
	return &c, err
}
