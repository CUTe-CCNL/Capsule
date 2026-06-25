package incusplugin

import (
	"context"
	"time"
)

type Config struct {
	SocketPath       string
	Project          string
	BackupDir        string
	OperationTimeout time.Duration
	ImageServer      string
	ImageProtocol    string
}

type ServerInfo struct {
	ServerName string `json:"server_name"`
	Version    string `json:"version"`
	Project    string `json:"project"`
}

type Instance struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Status string `json:"status"`
}

type Snapshot struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type OperationResult struct {
	Status      string `json:"status"`
	OperationID string `json:"operation_id,omitempty"`
}

type CreateInstanceRequest struct {
	Name     string                       `json:"name"`
	Type     string                       `json:"type"`
	Image    string                       `json:"image"`
	Profiles []string                     `json:"profiles,omitempty"`
	Network  string                       `json:"network,omitempty"`
	Start    bool                         `json:"start,omitempty"`
	Config   map[string]string            `json:"config,omitempty"`
	Devices  map[string]map[string]string `json:"devices,omitempty"`
}

type CreateSnapshotRequest struct {
	InstanceName string `json:"instance_name"`
	Name         string `json:"name"`
	Stateful     bool   `json:"stateful,omitempty"`
}

type RestoreSnapshotRequest struct {
	InstanceName string `json:"instance_name"`
	SnapshotName string `json:"snapshot_name"`
	Stateful     bool   `json:"stateful,omitempty"`
}

type DeleteSnapshotRequest struct {
	InstanceName string `json:"instance_name"`
	SnapshotName string `json:"snapshot_name"`
}

type ExportInstanceRequest struct {
	InstanceName  string `json:"instance_name"`
	FileName      string `json:"file_name,omitempty"`
	InstanceOnly  bool   `json:"instance_only,omitempty"`
	Optimized     bool   `json:"optimized,omitempty"`
	Compression   string `json:"compression,omitempty"`
	IncludeImages bool   `json:"include_images,omitempty"`
}

type ExportMetadata struct {
	FileName  string    `json:"file_name"`
	Instance  string    `json:"instance"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at,omitempty"`
}

type ImportExportRequest struct {
	FileName string `json:"file_name"`
	Name     string `json:"name,omitempty"`
}

type Client interface {
	Ping(context.Context) (ServerInfo, error)
	ListInstances(context.Context, string) ([]Instance, error)
	GetInstance(context.Context, string) (Instance, error)
	CreateInstance(context.Context, CreateInstanceRequest) (OperationResult, error)
	StartInstance(context.Context, string) (OperationResult, error)
	StopInstance(context.Context, string, bool) (OperationResult, error)
	RestartInstance(context.Context, string, bool) (OperationResult, error)
	DeleteInstance(context.Context, string, bool) (OperationResult, error)
	ListSnapshots(context.Context, string) ([]Snapshot, error)
	CreateSnapshot(context.Context, CreateSnapshotRequest) (OperationResult, error)
	RestoreSnapshot(context.Context, RestoreSnapshotRequest) (OperationResult, error)
	DeleteSnapshot(context.Context, DeleteSnapshotRequest) (OperationResult, error)
	ExportInstance(context.Context, ExportInstanceRequest) (ExportMetadata, error)
	ListExports(context.Context) ([]ExportMetadata, error)
	ImportExport(context.Context, ImportExportRequest) (OperationResult, error)
	DeleteExport(context.Context, string) error
}
