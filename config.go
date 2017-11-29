package main

import (
	"time"

	"github.com/BurntSushi/toml"
)

// TrackerConfig holds configuration values for Tracker.
type TrackerConfig struct {
	ListenAddress   string   `toml:"listen_address"`
	ShutdownTimeout Duration `toml:"shutdown_timeout"`
	TempfileTooOld  Duration `toml:"tempfile_too_old"`
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
	DataDir              string   `toml:"datadir"`
	ListenAddress        string   `toml:"listen_address"`
	ListenAddressForRead string   `toml:"listen_address_for_read"`
	ShutdownTimeout      Duration `toml:"shutdown_timeout"`
	CleanDiskRunPeriod   Duration `toml:"clean_disk_run_period"`
	CleanDiskFileTTL     Duration `toml:"clean_disk_file_ttl"`
}

// ClientConfig holds configuration values for Client.
type ClientConfig struct {
	TrackerURL   string    `toml:"tracker_url"`
	ChunkSize    ChunkSize `toml:"chunk_size"`
	SendTimeout  Duration  `toml:"send_timeout"`
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
		ShutdownTimeout: Duration(3 * time.Second),
		TempfileTooOld:  Duration(24 * time.Hour),
	},
	Server: ServerConfig{
		DataDir:              "/srv/efes/dev1",
		ListenAddress:        "0.0.0.0:8501",
		ListenAddressForRead: "0.0.0.0:8500",
		ShutdownTimeout:      Duration(10 * time.Second),
		CleanDiskFileTTL:     Duration(24 * time.Hour),
		CleanDiskRunPeriod:   Duration(7 * 24 * time.Hour),
	},
	Client: ClientConfig{
		TrackerURL:   "http://127.0.0.1:8001",
		ChunkSize:    50 * M,
		SendTimeout:  Duration(10 * time.Second),
		ShowProgress: true,
	},
	Database: DatabaseConfig{
		DSN: "test:test@(127.0.0.1:3306)/efes",
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
