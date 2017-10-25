package config

import "github.com/BurntSushi/toml"

// TrackerConfig holds configuration values for Tracker.
type TrackerConfig struct {
	Debug bool   `toml:"debug"`
	DBDSN string `toml:"db_dsn"`
}

// Config holds configuration values for all Efes components.
type Config struct {
	Tracker TrackerConfig
}

// New parses a TOML file and returns new Config.
func New(configFile string) (*Config, error) {
	var c Config
	_, err := toml.DecodeFile(configFile, &c)
	return &c, err
}
