package main

import "github.com/BurntSushi/toml"

// TrackerConfig holds configuration values for Tracker.
type TrackerConfig struct {
	ListenAddress   string `toml:"listen_address"`
	ShutdownTimeout uint32 `toml:"shutdown_timeout"`
	TempfileTooOld  uint32 `toml:"tempfile_too_old"`
}

// DatabaseConfig holds configuration values for database.
type DatabaseConfig struct {
	DSN string `toml:"dsn"`
}

// AMQPConfig holds configuration values for message broker.
type AMQPConfig struct {
	URL string `toml:"url"`
}

// ServerConfig holds configuration values for Server.
type ServerConfig struct {
	DataDir              string `toml:"datadir"`
	ListenAddress        string `toml:"listen_address"`
	ListenAddressForRead string `toml:"listen_address_for_read"`
	ShutdownTimeout      uint32 `toml:"shutdown_timeout"`
	CleanDiskRunPeriod   int    `toml:"clean_disk_run_period"`
	CleanDiskFileTTL     int    `toml:"clean_disk_file_ttl"`
}

// ClientConfig holds configuration values for Client.
type ClientConfig struct {
	TrackerURL   string    `toml:"tracker_url"`
	ChunkSize    ChunkSize `toml:"chunk_size"`
	ShowProgress bool      `toml:"show_progress"`
}

// Config holds configuration values for all Efes components.
type Config struct {
	Debug    bool
	Tracker  TrackerConfig
	Server   ServerConfig
	Client   ClientConfig
	Database DatabaseConfig
	AMQP     AMQPConfig
}

var defaultConfig = Config{
	Tracker: TrackerConfig{
		ListenAddress:   "0.0.0.0:8001",
		ShutdownTimeout: 3000,
		TempfileTooOld:  86400000,
	},
	Server: ServerConfig{
		DataDir:              "/srv/efes/dev1",
		ListenAddress:        "0.0.0.0:8501",
		ListenAddressForRead: "0.0.0.0:8500",
		ShutdownTimeout:      10000,
		CleanDiskFileTTL:     43200,
		CleanDiskRunPeriod:   259200,
	},
	Client: ClientConfig{
		TrackerURL:   "http://127.0.0.1:8001",
		ChunkSize:    100 * 1024 * 1024,
		ShowProgress: true,
	},
	Database: DatabaseConfig{
		DSN: "test:test@(127.0.0.1:3306)/mogilefs",
	},
	AMQP: AMQPConfig{
		URL: "amqp://guest:guest@127.0.0.1:5672/",
	},
}

// ReadConfig parses a TOML file and returns new Config.
func ReadConfig(configFile string) (*Config, error) {
	c := defaultConfig
	_, err := toml.DecodeFile(configFile, &c)
	return &c, err
}
