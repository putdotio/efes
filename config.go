package main

import (
	"time"

	"github.com/BurntSushi/toml"
)

// TrackerConfig holds configuration values for Tracker.
type TrackerConfig struct {
	ListenAddress           string   `toml:"listen_address"`
	ListenAddressForMetrics string   `toml:"listen_address_for_metrics"`
	ShutdownTimeout         Duration `toml:"shutdown_timeout"`
	TempfileTooOld          Duration `toml:"tempfile_too_old"`
}

// DatabaseConfig holds configuration values for database.
type DatabaseConfig struct {
	DSN             string   `toml:"dsn"`
	ConnMaxLifetime Duration `toml:"conn_max_lifetime"`
	MaxIdleConns    int      `toml:"max_idle_conns"`
	MaxOpenConns    int      `toml:"max_open_conns"`
}

// AMQPConfig holds configuration values for message broker.
type AMQPConfig struct {
	URL string `toml:"url"`
}

// ServerConfig holds configuration values for Server.
type ServerConfig struct {
	DataDir                 string   `toml:"datadir"`
	ListenAddressForWrite   string   `toml:"listen_address_for_write"`
	ListenAddressForRead    string   `toml:"listen_address_for_read"`
	ListenAddressForMetrics string   `toml:"listen_address_for_metrics"`
	ShutdownTimeout         Duration `toml:"shutdown_timeout"`
	CleanDiskRunPeriod      Duration `toml:"clean_disk_run_period"`
	CleanDiskFileTTL        Duration `toml:"clean_disk_file_ttl"`
	CleanDiskDryRun         bool     `toml:"clean_disk_dry_run"`
	CleanDeviceRunPeriod    Duration `toml:"clean_device_run_period"`
	CleanDeviceDryRun       bool     `toml:"clean_device_dry_run"`
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
	Debug     bool   `toml:"debug"`
	SentryDSN string `toml:"sentry_dsn"`
	Tracker   TrackerConfig
	Server    ServerConfig
	Client    ClientConfig
	Database  DatabaseConfig
	AMQP      AMQPConfig
}

var defaultConfig = Config{
	Tracker: TrackerConfig{
		ListenAddress:   "0.0.0.0:8001",
		ShutdownTimeout: Duration(3 * time.Second),
		TempfileTooOld:  Duration(24 * time.Hour),
	},
	Server: ServerConfig{
		DataDir:               "/srv/efes/dev1",
		ListenAddressForWrite: "0.0.0.0:8501",
		ListenAddressForRead:  "0.0.0.0:8500",
		ShutdownTimeout:       Duration(10 * time.Second),
		CleanDiskFileTTL:      Duration(24 * time.Hour),
		CleanDiskRunPeriod:    Duration(7 * 24 * time.Hour),
		CleanDeviceRunPeriod:  Duration(7 * 24 * time.Hour),
	},
	Client: ClientConfig{
		TrackerURL:   "http://127.0.0.1:8001",
		ChunkSize:    50 * M,
		SendTimeout:  Duration(10 * time.Second),
		ShowProgress: true,
	},
	Database: DatabaseConfig{
		DSN:             "test:test@(127.0.0.1:3306)/efes",
		ConnMaxLifetime: Duration(30 * time.Second),
		MaxIdleConns:    5,
		MaxOpenConns:    15,
	},
	AMQP: AMQPConfig{
		URL: "amqp://guest:guest@127.0.0.1:5672/",
	},
}

func NewConfig() *Config {
	c := new(Config)
	*c = defaultConfig
	return c
}

// ReadFile parses a TOML file and returns new Config.
func (c *Config) ReadFile(name string) error {
	_, err := toml.DecodeFile(name, c)
	return err
}
