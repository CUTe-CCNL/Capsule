package incusplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	incus "github.com/lxc/incus/v7/client"
	"github.com/lxc/incus/v7/shared/api"
)

type incusServer interface {
	GetServer() (*api.Server, string, error)
	GetInstances(api.InstanceType) ([]api.Instance, error)
	GetInstance(string) (*api.Instance, string, error)
	CreateInstance(api.InstancesPost) (incus.Operation, error)
	UpdateInstanceState(string, api.InstanceStatePut, string) (incus.Operation, error)
	DeleteInstance(string) (incus.Operation, error)
	GetInstanceSnapshots(string) ([]api.InstanceSnapshot, error)
	CreateInstanceSnapshot(string, api.InstanceSnapshotsPost) (incus.Operation, error)
	UpdateInstance(string, api.InstancePut, string) (incus.Operation, error)
	DeleteInstanceSnapshot(string, string) (incus.Operation, error)
	CreateInstanceBackup(string, api.InstanceBackupsPost) (incus.Operation, error)
	GetInstanceBackupFile(string, string, *incus.BackupFileRequest) (*incus.BackupFileResponse, error)
	DeleteInstanceBackup(string, string) (incus.Operation, error)
	CreateInstanceFromBackup(incus.InstanceBackupArgs) (incus.Operation, error)
}

type IncusClient struct {
	config Config
	server incusServer
}

func NewIncusClient(config Config) (Client, error) {
	server, err := incus.ConnectIncusUnixWithContext(context.Background(), config.SocketPath, nil)
	if err != nil {
		return nil, fmt.Errorf("connect incus unix socket: %w", err)
	}
	if config.Project != "" {
		server = server.UseProject(config.Project)
	}

	return newIncusClient(config, server), nil
}

func newIncusClient(config Config, server incusServer) *IncusClient {
	return &IncusClient{config: config, server: server}
}

func (c *IncusClient) Ping(ctx context.Context) (ServerInfo, error) {
	server, _, err := c.server.GetServer()
	if err != nil {
		return ServerInfo{}, err
	}

	project := server.Environment.Project
	if project == "" {
		project = c.config.Project
	}

	name := server.Environment.ServerName
	if name == "" {
		name = server.Environment.Server
	}

	return ServerInfo{
		ServerName: name,
		Version:    server.Environment.ServerVersion,
		Project:    project,
	}, nil
}

func (c *IncusClient) ListInstances(ctx context.Context, instanceType string) ([]Instance, error) {
	incusType, err := parseInstanceType(instanceType, true)
	if err != nil {
		return nil, err
	}

	instances, err := c.server.GetInstances(incusType)
	if err != nil {
		return nil, err
	}

	result := make([]Instance, 0, len(instances))
	for _, instance := range instances {
		result = append(result, mapInstance(instance))
	}

	return result, nil
}

func (c *IncusClient) GetInstance(ctx context.Context, name string) (Instance, error) {
	instance, _, err := c.server.GetInstance(name)
	if err != nil {
		return Instance{}, err
	}

	return mapInstance(*instance), nil
}

func (c *IncusClient) CreateInstance(ctx context.Context, req CreateInstanceRequest) (OperationResult, error) {
	instanceType, err := parseInstanceType(req.Type, false)
	if err != nil {
		return OperationResult{}, err
	}

	post := api.InstancesPost{
		Name:  req.Name,
		Type:  instanceType,
		Start: req.Start,
		Source: api.InstanceSource{
			Type:     "image",
			Alias:    req.Image,
			Server:   c.config.ImageServer,
			Protocol: c.config.ImageProtocol,
		},
		InstancePut: api.InstancePut{
			Profiles: req.Profiles,
			Config:   api.ConfigMap(req.Config),
			Devices:  api.DevicesMap(req.Devices),
		},
	}

	op, err := c.server.CreateInstance(post)
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) StartInstance(ctx context.Context, name string) (OperationResult, error) {
	return c.changeInstanceState(ctx, name, "start", false)
}

func (c *IncusClient) StopInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	return c.changeInstanceState(ctx, name, "stop", force)
}

func (c *IncusClient) RestartInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	return c.changeInstanceState(ctx, name, "restart", force)
}

func (c *IncusClient) DeleteInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	if force {
		instance, _, err := c.server.GetInstance(name)
		if err != nil {
			return OperationResult{}, err
		}
		if instance.IsActive() {
			if _, err := c.StopInstance(ctx, name, true); err != nil {
				return OperationResult{}, err
			}
		}
	}

	op, err := c.server.DeleteInstance(name)
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) ListSnapshots(ctx context.Context, instanceName string) ([]Snapshot, error) {
	snapshots, err := c.server.GetInstanceSnapshots(instanceName)
	if err != nil {
		return nil, err
	}

	result := make([]Snapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		result = append(result, Snapshot{Name: snapshot.Name, CreatedAt: snapshot.CreatedAt})
	}

	return result, nil
}

func (c *IncusClient) CreateSnapshot(ctx context.Context, req CreateSnapshotRequest) (OperationResult, error) {
	op, err := c.server.CreateInstanceSnapshot(req.InstanceName, api.InstanceSnapshotsPost{
		Name:     req.Name,
		Stateful: req.Stateful,
	})
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) RestoreSnapshot(ctx context.Context, req RestoreSnapshotRequest) (OperationResult, error) {
	instance, etag, err := c.server.GetInstance(req.InstanceName)
	if err != nil {
		return OperationResult{}, err
	}

	put := instance.Writable()
	put.Restore = req.SnapshotName
	put.Stateful = req.Stateful

	op, err := c.server.UpdateInstance(req.InstanceName, put, etag)
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) DeleteSnapshot(ctx context.Context, req DeleteSnapshotRequest) (OperationResult, error) {
	op, err := c.server.DeleteInstanceSnapshot(req.InstanceName, req.SnapshotName)
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) ExportInstance(ctx context.Context, req ExportInstanceRequest) (ExportMetadata, error) {
	now := time.Now()
	if req.FileName == "" {
		req.FileName = defaultExportFileName(req.InstanceName, now)
	}
	if err := validateFileName(req.FileName); err != nil {
		return ExportMetadata{}, err
	}
	if err := os.MkdirAll(c.config.BackupDir, 0o700); err != nil {
		return ExportMetadata{}, fmt.Errorf("create backup directory: %w", err)
	}

	backupName := defaultBackupName(req.InstanceName, now)
	op, err := c.server.CreateInstanceBackup(req.InstanceName, api.InstanceBackupsPost{
		Name:                 backupName,
		InstanceOnly:         req.InstanceOnly,
		OptimizedStorage:     req.Optimized,
		CompressionAlgorithm: req.Compression,
	})
	if err != nil {
		return ExportMetadata{}, err
	}
	if _, err := c.waitForOperation(ctx, op); err != nil {
		return ExportMetadata{}, err
	}

	filePath := c.exportPath(req.FileName)
	metadata, err := c.downloadBackup(ctx, req.InstanceName, backupName, filePath, req.FileName)
	cleanupErr := c.deleteRemoteBackup(ctx, req.InstanceName, backupName)
	if err != nil {
		_ = os.Remove(filePath)
		return ExportMetadata{}, err
	}
	if cleanupErr != nil {
		return ExportMetadata{}, cleanupErr
	}

	metadata.Instance = req.InstanceName
	if err := c.writeExportMetadata(metadata); err != nil {
		return ExportMetadata{}, err
	}

	return metadata, nil
}

func (c *IncusClient) ListExports(ctx context.Context) ([]ExportMetadata, error) {
	entries, err := os.ReadDir(c.config.BackupDir)
	if errors.Is(err, os.ErrNotExist) {
		return []ExportMetadata{}, nil
	}
	if err != nil {
		return nil, err
	}

	exports := make([]ExportMetadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			return nil, err
		}

		metadata := ExportMetadata{
			FileName:  entry.Name(),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime(),
		}

		if saved, err := c.readExportMetadata(entry.Name()); err == nil {
			metadata.Instance = saved.Instance
			if !saved.CreatedAt.IsZero() {
				metadata.CreatedAt = saved.CreatedAt
			}
		}

		exports = append(exports, metadata)
	}

	sort.Slice(exports, func(i, j int) bool {
		if exports[i].CreatedAt.Equal(exports[j].CreatedAt) {
			return exports[i].FileName < exports[j].FileName
		}
		return exports[i].CreatedAt.After(exports[j].CreatedAt)
	})

	return exports, nil
}

func (c *IncusClient) ImportExport(ctx context.Context, req ImportExportRequest) (OperationResult, error) {
	if err := validateFileName(req.FileName); err != nil {
		return OperationResult{}, err
	}

	file, err := os.Open(c.exportPath(req.FileName))
	if err != nil {
		return OperationResult{}, err
	}
	defer func() { _ = file.Close() }()

	op, err := c.server.CreateInstanceFromBackup(incus.InstanceBackupArgs{
		BackupFile: file,
		Name:       req.Name,
	})
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) DeleteExport(ctx context.Context, fileName string) error {
	if err := validateFileName(fileName); err != nil {
		return err
	}

	if err := os.Remove(c.exportPath(fileName)); err != nil {
		return err
	}
	if err := os.Remove(c.exportMetadataPath(fileName)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (c *IncusClient) changeInstanceState(ctx context.Context, name string, action string, force bool) (OperationResult, error) {
	timeout := int(c.config.OperationTimeout.Seconds())
	if timeout < 0 {
		timeout = 0
	}

	op, err := c.server.UpdateInstanceState(name, api.InstanceStatePut{
		Action:  action,
		Timeout: timeout,
		Force:   force,
	}, "")
	if err != nil {
		return OperationResult{}, err
	}

	return c.waitForOperation(ctx, op)
}

func (c *IncusClient) waitForOperation(ctx context.Context, op incus.Operation) (OperationResult, error) {
	if op == nil {
		return OperationResult{}, errors.New("incus operation is nil")
	}

	waitCtx, cancel := c.operationContext(ctx)
	defer cancel()

	if err := op.WaitContext(waitCtx); err != nil {
		return OperationResult{}, err
	}

	operation := op.Get()
	status := strings.ToLower(operation.Status)
	if status == "" {
		status = "success"
	}

	return OperationResult{Status: status, OperationID: operation.ID}, nil
}

func (c *IncusClient) operationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.config.OperationTimeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, c.config.OperationTimeout)
}

func (c *IncusClient) downloadBackup(ctx context.Context, instanceName string, backupName string, filePath string, fileName string) (ExportMetadata, error) {
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return ExportMetadata{}, err
	}

	response, downloadErr := c.server.GetInstanceBackupFile(instanceName, backupName, &incus.BackupFileRequest{BackupFile: file})
	closeErr := file.Close()
	if downloadErr != nil {
		return ExportMetadata{}, downloadErr
	}
	if closeErr != nil {
		return ExportMetadata{}, closeErr
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return ExportMetadata{}, err
	}

	size := info.Size()
	if response != nil && response.Size > 0 {
		size = response.Size
	}

	return ExportMetadata{
		FileName:  fileName,
		SizeBytes: size,
		CreatedAt: info.ModTime(),
	}, nil
}

func (c *IncusClient) deleteRemoteBackup(ctx context.Context, instanceName string, backupName string) error {
	op, err := c.server.DeleteInstanceBackup(instanceName, backupName)
	if err != nil {
		return err
	}
	_, err = c.waitForOperation(ctx, op)
	return err
}

func (c *IncusClient) exportPath(fileName string) string {
	return filepath.Join(c.config.BackupDir, fileName)
}

func (c *IncusClient) exportMetadataPath(fileName string) string {
	return filepath.Join(c.config.BackupDir, fileName+".json")
}

func (c *IncusClient) writeExportMetadata(metadata ExportMetadata) error {
	body, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	return os.WriteFile(c.exportMetadataPath(metadata.FileName), body, 0o600)
}

func (c *IncusClient) readExportMetadata(fileName string) (ExportMetadata, error) {
	body, err := os.ReadFile(c.exportMetadataPath(fileName))
	if err != nil {
		return ExportMetadata{}, err
	}

	var metadata ExportMetadata
	if err := json.Unmarshal(body, &metadata); err != nil {
		return ExportMetadata{}, err
	}

	return metadata, nil
}

func parseInstanceType(value string, allowAny bool) (api.InstanceType, error) {
	switch value {
	case "":
		if allowAny {
			return api.InstanceTypeAny, nil
		}
		return api.InstanceTypeContainer, nil
	case "container":
		return api.InstanceTypeContainer, nil
	case "virtual-machine":
		return api.InstanceTypeVM, nil
	default:
		return "", fmt.Errorf("unsupported instance type %q", value)
	}
}

func mapInstance(instance api.Instance) Instance {
	return Instance{
		Name:   instance.Name,
		Type:   instance.Type,
		Status: instance.Status,
	}
}

func defaultExportFileName(instanceName string, now time.Time) string {
	return fmt.Sprintf("%s-%s.tar.gz", safeFilePrefix(instanceName), now.UTC().Format("20060102-150405-000000000"))
}

func defaultBackupName(instanceName string, now time.Time) string {
	return fmt.Sprintf("host-agent-%s-%s", safeFilePrefix(instanceName), now.UTC().Format("20060102-150405-000000000"))
}

func safeFilePrefix(value string) string {
	var builder strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z':
			builder.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char)
		case char >= '0' && char <= '9':
			builder.WriteRune(char)
		case char == '-' || char == '_':
			builder.WriteRune(char)
		default:
			builder.WriteRune('-')
		}
	}

	prefix := strings.Trim(builder.String(), "-_")
	if prefix == "" {
		return "instance"
	}
	return prefix
}
