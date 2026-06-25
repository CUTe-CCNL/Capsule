package incusplugin

import (
	"fmt"
	"os"
	"time"
)

const (
	defaultProject          = "default"
	defaultBackupDir        = "/var/lib/host-agent/incus-backups"
	defaultOperationTimeout = 10 * time.Minute
	defaultImageServer      = "https://images.linuxcontainers.org"
	defaultImageProtocol    = "simplestreams"
)

func ConfigFromEnv() (Config, error) {
	cfg := Config{
		SocketPath:       os.Getenv("INCUS_SOCKET_PATH"),
		Project:          envOrDefault("INCUS_PROJECT", defaultProject),
		BackupDir:        envOrDefault("INCUS_BACKUP_DIR", defaultBackupDir),
		OperationTimeout: defaultOperationTimeout,
		ImageServer:      envOrDefault("INCUS_IMAGE_SERVER", defaultImageServer),
		ImageProtocol:    envOrDefault("INCUS_IMAGE_PROTOCOL", defaultImageProtocol),
	}

	if value := os.Getenv("INCUS_OPERATION_TIMEOUT"); value != "" {
		timeout, err := time.ParseDuration(value)
		if err != nil {
			return Config{}, fmt.Errorf("parse INCUS_OPERATION_TIMEOUT: %w", err)
		}
		cfg.OperationTimeout = timeout
	}

	return cfg, nil
}

func envOrDefault(name, defaultValue string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultValue
	}
	return value
}
