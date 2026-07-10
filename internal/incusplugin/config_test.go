package incusplugin

import (
	"testing"
	"time"
)

func TestConfigFromEnvUsesDefaults(t *testing.T) {
	t.Setenv("INCUS_SOCKET_PATH", "")
	t.Setenv("INCUS_PROJECT", "")
	t.Setenv("INCUS_BACKUP_DIR", "")
	t.Setenv("INCUS_OPERATION_TIMEOUT", "")
	t.Setenv("INCUS_IMAGE_SERVER", "")
	t.Setenv("INCUS_IMAGE_PROTOCOL", "")
	t.Setenv("INCUS_STORAGE_POOL", "")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if cfg.SocketPath != "" {
		t.Fatalf("SocketPath = %q, want empty default", cfg.SocketPath)
	}
	if cfg.Project != "default" {
		t.Fatalf("Project = %q, want default", cfg.Project)
	}
	if cfg.BackupDir != "/var/lib/host-agent/incus-backups" {
		t.Fatalf("BackupDir = %q, want default backup dir", cfg.BackupDir)
	}
	if cfg.OperationTimeout != 10*time.Minute {
		t.Fatalf("OperationTimeout = %s, want 10m", cfg.OperationTimeout)
	}
	if cfg.ImageServer != "https://images.linuxcontainers.org" {
		t.Fatalf("ImageServer = %q, want images server", cfg.ImageServer)
	}
	if cfg.ImageProtocol != "simplestreams" {
		t.Fatalf("ImageProtocol = %q, want simplestreams", cfg.ImageProtocol)
	}
	if cfg.StoragePool != "default" {
		t.Fatalf("StoragePool = %q, want default", cfg.StoragePool)
	}
}

func TestConfigFromEnvAllowsOverrides(t *testing.T) {
	t.Setenv("INCUS_SOCKET_PATH", "/run/incus/custom.sock")
	t.Setenv("INCUS_PROJECT", "lab")
	t.Setenv("INCUS_BACKUP_DIR", "/srv/backups")
	t.Setenv("INCUS_OPERATION_TIMEOUT", "2m30s")
	t.Setenv("INCUS_IMAGE_SERVER", "https://mirror.example.test")
	t.Setenv("INCUS_IMAGE_PROTOCOL", "oci")
	t.Setenv("INCUS_STORAGE_POOL", "fast")

	cfg, err := ConfigFromEnv()
	if err != nil {
		t.Fatalf("ConfigFromEnv() error = %v", err)
	}

	if cfg.SocketPath != "/run/incus/custom.sock" {
		t.Fatalf("SocketPath = %q, want override", cfg.SocketPath)
	}
	if cfg.Project != "lab" {
		t.Fatalf("Project = %q, want lab", cfg.Project)
	}
	if cfg.BackupDir != "/srv/backups" {
		t.Fatalf("BackupDir = %q, want /srv/backups", cfg.BackupDir)
	}
	if cfg.OperationTimeout != 150*time.Second {
		t.Fatalf("OperationTimeout = %s, want 2m30s", cfg.OperationTimeout)
	}
	if cfg.ImageServer != "https://mirror.example.test" {
		t.Fatalf("ImageServer = %q, want mirror", cfg.ImageServer)
	}
	if cfg.ImageProtocol != "oci" {
		t.Fatalf("ImageProtocol = %q, want oci", cfg.ImageProtocol)
	}
	if cfg.StoragePool != "fast" {
		t.Fatalf("StoragePool = %q, want fast", cfg.StoragePool)
	}
}

func TestConfigFromEnvRejectsInvalidTimeout(t *testing.T) {
	t.Setenv("INCUS_OPERATION_TIMEOUT", "soon")

	if _, err := ConfigFromEnv(); err == nil {
		t.Fatal("ConfigFromEnv() error = nil, want invalid duration error")
	}
}
