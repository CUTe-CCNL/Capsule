package incusplugin

import (
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	incus "github.com/lxc/incus/v7/client"
	"github.com/lxc/incus/v7/shared/api"
)

func TestIncusClientCreateInstanceMapsSDKRequest(t *testing.T) {
	server := &fakeIncusServer{}
	client := newIncusClient(Config{
		ImageServer:      "https://images.example.test",
		ImageProtocol:    "simplestreams",
		OperationTimeout: time.Minute,
	}, server)

	result, err := client.CreateInstance(context.Background(), CreateInstanceRequest{
		Name:     "web-1",
		Type:     "virtual-machine",
		Image:    "ubuntu/24.04",
		Profiles: []string{"default"},
		Start:    true,
		Config: map[string]string{
			"limits.cpu": "4",
		},
		Devices: map[string]map[string]string{
			"eth0": {"type": "nic", "network": "ovn-prod"},
		},
	})
	if err != nil {
		t.Fatalf("CreateInstance() error = %v", err)
	}

	if result.Status != "success" || result.OperationID == "" {
		t.Fatalf("result = %+v, want success with operation ID", result)
	}
	if len(server.operations) != 1 || !server.operations[0].waited {
		t.Fatalf("operation wait state = %+v", server.operations)
	}

	req := server.createdInstance
	if req.Name != "web-1" || req.Type != api.InstanceTypeVM || !req.Start {
		t.Fatalf("created request = %+v", req)
	}
	if req.Source.Type != "image" || req.Source.Alias != "ubuntu/24.04" {
		t.Fatalf("source = %+v, want image alias", req.Source)
	}
	if req.Source.Server != "https://images.example.test" || req.Source.Protocol != "simplestreams" {
		t.Fatalf("source remote = %+v", req.Source)
	}
	if req.Profiles[0] != "default" {
		t.Fatalf("profiles = %+v, want default", req.Profiles)
	}
	if req.Config["limits.cpu"] != "4" {
		t.Fatalf("limits.cpu = %q, want 4", req.Config["limits.cpu"])
	}
	if req.Devices["eth0"]["network"] != "ovn-prod" {
		t.Fatalf("eth0 network = %q, want ovn-prod", req.Devices["eth0"]["network"])
	}
}

func TestIncusClientRestoreSnapshotUsesWritableInstance(t *testing.T) {
	server := &fakeIncusServer{
		instance: &api.Instance{
			Name: "web-1",
			Type: string(api.InstanceTypeContainer),
			InstancePut: api.InstancePut{
				Config:   api.ConfigMap{"limits.cpu": "2"},
				Profiles: []string{"default"},
			},
		},
		etag: "etag-1",
	}
	client := newIncusClient(Config{OperationTimeout: time.Minute}, server)

	result, err := client.RestoreSnapshot(context.Background(), RestoreSnapshotRequest{
		InstanceName: "web-1",
		SnapshotName: "before-upgrade",
		Stateful:     true,
	})
	if err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}

	if result.Status != "success" {
		t.Fatalf("Status = %q, want success", result.Status)
	}
	if server.updatedInstanceName != "web-1" || server.updatedETag != "etag-1" {
		t.Fatalf("updated target = %s etag %s", server.updatedInstanceName, server.updatedETag)
	}
	if server.updatedInstance.Restore != "before-upgrade" || !server.updatedInstance.Stateful {
		t.Fatalf("updated instance = %+v", server.updatedInstance)
	}
	if server.updatedInstance.Config["limits.cpu"] != "2" {
		t.Fatalf("updated config = %+v, want original writable config", server.updatedInstance.Config)
	}
}

func TestIncusClientExportDownloadsBackupAndListsMetadata(t *testing.T) {
	server := &fakeIncusServer{backupContent: []byte("backup-data")}
	client := newIncusClient(Config{
		BackupDir:        t.TempDir(),
		OperationTimeout: time.Minute,
	}, server)

	metadata, err := client.ExportInstance(context.Background(), ExportInstanceRequest{
		InstanceName:  "web-1",
		FileName:      "web-1.tar.gz",
		InstanceOnly:  true,
		Optimized:     true,
		Compression:   "gzip",
		IncludeImages: true,
	})
	if err != nil {
		t.Fatalf("ExportInstance() error = %v", err)
	}

	if metadata.FileName != "web-1.tar.gz" || metadata.Instance != "web-1" || metadata.SizeBytes != int64(len("backup-data")) {
		t.Fatalf("metadata = %+v", metadata)
	}
	if server.backupInstance != "web-1" {
		t.Fatalf("backup instance = %q, want web-1", server.backupInstance)
	}
	if !server.backupPost.InstanceOnly || !server.backupPost.OptimizedStorage || server.backupPost.CompressionAlgorithm != "gzip" {
		t.Fatalf("backup post = %+v", server.backupPost)
	}
	if server.downloadedBackup == "" || server.deletedBackup != server.downloadedBackup {
		t.Fatalf("downloaded backup = %q, deleted backup = %q", server.downloadedBackup, server.deletedBackup)
	}

	body, err := os.ReadFile(client.exportPath("web-1.tar.gz"))
	if err != nil {
		t.Fatalf("read export: %v", err)
	}
	if string(body) != "backup-data" {
		t.Fatalf("export body = %q, want backup-data", string(body))
	}

	exports, err := client.ListExports(context.Background())
	if err != nil {
		t.Fatalf("ListExports() error = %v", err)
	}
	if len(exports) != 1 || exports[0].FileName != "web-1.tar.gz" || exports[0].Instance != "web-1" {
		t.Fatalf("exports = %+v", exports)
	}
}

func TestIncusClientImportExportPassesBackupReaderAndName(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/web-1.tar.gz", []byte("backup-data"), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}

	server := &fakeIncusServer{}
	client := newIncusClient(Config{
		BackupDir:        dir,
		OperationTimeout: time.Minute,
	}, server)

	result, err := client.ImportExport(context.Background(), ImportExportRequest{
		FileName: "web-1.tar.gz",
		Name:     "web-1-restored",
	})
	if err != nil {
		t.Fatalf("ImportExport() error = %v", err)
	}

	if result.Status != "success" {
		t.Fatalf("Status = %q, want success", result.Status)
	}
	if server.importedName != "web-1-restored" {
		t.Fatalf("imported name = %q, want web-1-restored", server.importedName)
	}
	if string(server.importedContent) != "backup-data" {
		t.Fatalf("imported content = %q, want backup-data", string(server.importedContent))
	}
}

type fakeIncusServer struct {
	server api.Server

	instances []api.Instance
	instance  *api.Instance
	etag      string

	createdInstance api.InstancesPost
	stateName       string
	statePut        api.InstanceStatePut
	deletedInstance string

	snapshots           []api.InstanceSnapshot
	createdSnapshot     api.InstanceSnapshotsPost
	deletedSnapshotName string
	updatedInstanceName string
	updatedInstance     api.InstancePut
	updatedETag         string
	backupInstance      string
	backupPost          api.InstanceBackupsPost
	backupContent       []byte
	downloadedBackup    string
	deletedBackup       string
	importedName        string
	importedContent     []byte
	operations          []*fakeIncusOperation
}

func (f *fakeIncusServer) GetServer() (*api.Server, string, error) {
	return &f.server, "", nil
}

func (f *fakeIncusServer) GetInstances(instanceType api.InstanceType) ([]api.Instance, error) {
	return f.instances, nil
}

func (f *fakeIncusServer) GetInstance(name string) (*api.Instance, string, error) {
	if f.instance != nil {
		return f.instance, f.etag, nil
	}
	return &api.Instance{Name: name, Type: string(api.InstanceTypeContainer), Status: "Stopped"}, f.etag, nil
}

func (f *fakeIncusServer) CreateInstance(instance api.InstancesPost) (incus.Operation, error) {
	f.createdInstance = instance
	return f.newOperation(), nil
}

func (f *fakeIncusServer) UpdateInstanceState(name string, state api.InstanceStatePut, etag string) (incus.Operation, error) {
	f.stateName = name
	f.statePut = state
	return f.newOperation(), nil
}

func (f *fakeIncusServer) DeleteInstance(name string) (incus.Operation, error) {
	f.deletedInstance = name
	return f.newOperation(), nil
}

func (f *fakeIncusServer) GetInstanceSnapshots(instanceName string) ([]api.InstanceSnapshot, error) {
	return f.snapshots, nil
}

func (f *fakeIncusServer) CreateInstanceSnapshot(instanceName string, snapshot api.InstanceSnapshotsPost) (incus.Operation, error) {
	f.createdSnapshot = snapshot
	return f.newOperation(), nil
}

func (f *fakeIncusServer) UpdateInstance(name string, instance api.InstancePut, etag string) (incus.Operation, error) {
	f.updatedInstanceName = name
	f.updatedInstance = instance
	f.updatedETag = etag
	return f.newOperation(), nil
}

func (f *fakeIncusServer) DeleteInstanceSnapshot(instanceName string, name string) (incus.Operation, error) {
	f.deletedSnapshotName = name
	return f.newOperation(), nil
}

func (f *fakeIncusServer) CreateInstanceBackup(instanceName string, backup api.InstanceBackupsPost) (incus.Operation, error) {
	f.backupInstance = instanceName
	f.backupPost = backup
	return f.newOperation(), nil
}

func (f *fakeIncusServer) GetInstanceBackupFile(instanceName string, name string, req *incus.BackupFileRequest) (*incus.BackupFileResponse, error) {
	f.downloadedBackup = name
	if _, err := req.BackupFile.Write(f.backupContent); err != nil {
		return nil, err
	}
	return &incus.BackupFileResponse{Size: int64(len(f.backupContent))}, nil
}

func (f *fakeIncusServer) DeleteInstanceBackup(instanceName string, name string) (incus.Operation, error) {
	f.deletedBackup = name
	return f.newOperation(), nil
}

func (f *fakeIncusServer) CreateInstanceFromBackup(args incus.InstanceBackupArgs) (incus.Operation, error) {
	f.importedName = args.Name
	body, err := io.ReadAll(args.BackupFile)
	if err != nil {
		return nil, err
	}
	f.importedContent = body
	return f.newOperation(), nil
}

func (f *fakeIncusServer) newOperation() *fakeIncusOperation {
	op := &fakeIncusOperation{Operation: api.Operation{
		ID:     "op-test",
		Status: "Success",
	}}
	f.operations = append(f.operations, op)
	return op
}

type fakeIncusOperation struct {
	api.Operation
	waited bool
}

func (f *fakeIncusOperation) AddHandler(function func(api.Operation)) (*incus.EventTarget, error) {
	return nil, nil
}

func (f *fakeIncusOperation) Cancel() error {
	return nil
}

func (f *fakeIncusOperation) Get() api.Operation {
	return f.Operation
}

func (f *fakeIncusOperation) GetWebsocket(secret string) (*websocket.Conn, error) {
	return nil, nil
}

func (f *fakeIncusOperation) RemoveHandler(target *incus.EventTarget) error {
	return nil
}

func (f *fakeIncusOperation) Refresh() error {
	return nil
}

func (f *fakeIncusOperation) Wait() error {
	f.waited = true
	return nil
}

func (f *fakeIncusOperation) WaitContext(ctx context.Context) error {
	f.waited = true
	return nil
}
