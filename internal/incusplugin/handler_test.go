package incusplugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/CUTe-CCNL/host-agent/pkg/pluginipc"
)

type fakeClient struct {
	pingInfo ServerInfo

	createdInstance CreateInstanceRequest
	startedInstance string
	snapshots       []Snapshot
	createdSnapshot CreateSnapshotRequest
	restored        RestoreSnapshotRequest
	deletedSnapshot DeleteSnapshotRequest
	exported        ExportInstanceRequest
	imported        ImportExportRequest
	exports         []ExportMetadata
	err             error
}

func (f *fakeClient) Ping(ctx context.Context) (ServerInfo, error) {
	if f.err != nil {
		return ServerInfo{}, f.err
	}
	return f.pingInfo, nil
}

func (f *fakeClient) ListInstances(ctx context.Context, instanceType string) ([]Instance, error) {
	if f.err != nil {
		return nil, f.err
	}
	return []Instance{{Name: "web-1", Type: "container", Status: "Running"}}, nil
}

func (f *fakeClient) GetInstance(ctx context.Context, name string) (Instance, error) {
	if f.err != nil {
		return Instance{}, f.err
	}
	return Instance{Name: name, Type: "virtual-machine", Status: "Stopped"}, nil
}

func (f *fakeClient) CreateInstance(ctx context.Context, req CreateInstanceRequest) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.createdInstance = req
	return OperationResult{Status: "success", OperationID: "op-create"}, nil
}

func (f *fakeClient) StartInstance(ctx context.Context, name string) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.startedInstance = name
	return OperationResult{Status: "success", OperationID: "op-start"}, nil
}

func (f *fakeClient) StopInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	return OperationResult{Status: "success", OperationID: "op-stop"}, f.err
}

func (f *fakeClient) RestartInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	return OperationResult{Status: "success", OperationID: "op-restart"}, f.err
}

func (f *fakeClient) DeleteInstance(ctx context.Context, name string, force bool) (OperationResult, error) {
	return OperationResult{Status: "success", OperationID: "op-delete"}, f.err
}

func (f *fakeClient) ListSnapshots(ctx context.Context, instanceName string) ([]Snapshot, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.snapshots, nil
}

func (f *fakeClient) CreateSnapshot(ctx context.Context, req CreateSnapshotRequest) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.createdSnapshot = req
	return OperationResult{Status: "success", OperationID: "op-snapshot"}, nil
}

func (f *fakeClient) RestoreSnapshot(ctx context.Context, req RestoreSnapshotRequest) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.restored = req
	return OperationResult{Status: "success", OperationID: "op-restore"}, nil
}

func (f *fakeClient) DeleteSnapshot(ctx context.Context, req DeleteSnapshotRequest) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.deletedSnapshot = req
	return OperationResult{Status: "success", OperationID: "op-delete-snapshot"}, nil
}

func (f *fakeClient) ExportInstance(ctx context.Context, req ExportInstanceRequest) (ExportMetadata, error) {
	if f.err != nil {
		return ExportMetadata{}, f.err
	}
	f.exported = req
	return ExportMetadata{FileName: "web-1.tar.gz", Instance: req.InstanceName, SizeBytes: 42}, nil
}

func (f *fakeClient) ListExports(ctx context.Context) ([]ExportMetadata, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.exports, nil
}

func (f *fakeClient) ImportExport(ctx context.Context, req ImportExportRequest) (OperationResult, error) {
	if f.err != nil {
		return OperationResult{}, f.err
	}
	f.imported = req
	return OperationResult{Status: "success", OperationID: "op-import"}, nil
}

func (f *fakeClient) DeleteExport(ctx context.Context, fileName string) error {
	return f.err
}

func TestStatusReturnsIncusConnectivity(t *testing.T) {
	client := &fakeClient{pingInfo: ServerInfo{
		ServerName: "incus",
		Version:    "7.1",
		Project:    "default",
	}}
	handler := NewHandler(Config{Project: "default"}, client)

	response := handle(t, handler, http.MethodGet, "/status", nil)

	if response.StatusCode != http.StatusOK {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusOK)
	}
	body := decodeBody[map[string]interface{}](t, response)
	if body["status"] != "ok" {
		t.Fatalf("status = %v, want ok", body["status"])
	}
	if body["project"] != "default" {
		t.Fatalf("project = %v, want default", body["project"])
	}
}

func TestCreateInstanceBuildsRequestWithDefaultsAndOverrides(t *testing.T) {
	client := &fakeClient{}
	handler := NewHandler(Config{Project: "default"}, client)

	payload := map[string]interface{}{
		"name":     "web-1",
		"type":     "virtual-machine",
		"image":    "ubuntu/24.04",
		"profiles": []string{"default"},
		"network":  "ovn-prod",
		"cpu":      4,
		"memory":   "8GiB",
		"disk":     "80GiB",
		"start":    true,
		"config": map[string]string{
			"security.secureboot": "false",
		},
		"devices": map[string]map[string]string{
			"extra": {"type": "disk", "path": "/data", "pool": "default", "size": "20GiB"},
		},
	}

	response := handle(t, handler, http.MethodPost, "/instances", payload)

	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusAccepted)
	}
	req := client.createdInstance
	if req.Name != "web-1" || req.Type != "virtual-machine" || req.Image != "ubuntu/24.04" {
		t.Fatalf("created request = %+v", req)
	}
	if !req.Start {
		t.Fatal("Start = false, want true")
	}
	if req.Config["limits.cpu"] != "4" {
		t.Fatalf("limits.cpu = %q, want 4", req.Config["limits.cpu"])
	}
	if req.Config["limits.memory"] != "8GiB" {
		t.Fatalf("limits.memory = %q, want 8GiB", req.Config["limits.memory"])
	}
	if req.Config["security.secureboot"] != "false" {
		t.Fatalf("security.secureboot = %q, want false", req.Config["security.secureboot"])
	}
	if req.Devices["root"]["size"] != "80GiB" {
		t.Fatalf("root disk size = %q, want 80GiB", req.Devices["root"]["size"])
	}
	if req.Devices["eth0"]["network"] != "ovn-prod" {
		t.Fatalf("eth0 network = %q, want ovn-prod", req.Devices["eth0"]["network"])
	}
	if req.Devices["extra"]["path"] != "/data" {
		t.Fatalf("extra disk path = %q, want /data", req.Devices["extra"]["path"])
	}
}

func TestInstanceSnapshotAndExportRoutes(t *testing.T) {
	client := &fakeClient{
		snapshots: []Snapshot{{Name: "before-upgrade", CreatedAt: time.Unix(1700000000, 0)}},
		exports:   []ExportMetadata{{FileName: "web-1.tar.gz", Instance: "web-1", SizeBytes: 42}},
	}
	handler := NewHandler(Config{Project: "default", BackupDir: "/backups"}, client)

	createSnapshot := handle(t, handler, http.MethodPost, "/instances/web-1/snapshots", map[string]interface{}{
		"name":     "before-upgrade",
		"stateful": true,
	})
	if createSnapshot.StatusCode != http.StatusAccepted {
		t.Fatalf("create snapshot status = %d, want %d", createSnapshot.StatusCode, http.StatusAccepted)
	}
	if client.createdSnapshot.InstanceName != "web-1" || client.createdSnapshot.Name != "before-upgrade" || !client.createdSnapshot.Stateful {
		t.Fatalf("created snapshot = %+v", client.createdSnapshot)
	}

	restore := handle(t, handler, http.MethodPost, "/instances/web-1/snapshots/before-upgrade/restore", map[string]interface{}{"stateful": true})
	if restore.StatusCode != http.StatusAccepted {
		t.Fatalf("restore snapshot status = %d, want %d", restore.StatusCode, http.StatusAccepted)
	}
	if client.restored.InstanceName != "web-1" || client.restored.SnapshotName != "before-upgrade" || !client.restored.Stateful {
		t.Fatalf("restored snapshot = %+v", client.restored)
	}

	export := handle(t, handler, http.MethodPost, "/instances/web-1/exports", map[string]interface{}{
		"file_name":     "web-1.tar.gz",
		"instance_only": true,
	})
	if export.StatusCode != http.StatusAccepted {
		t.Fatalf("export status = %d, want %d", export.StatusCode, http.StatusAccepted)
	}
	if client.exported.InstanceName != "web-1" || client.exported.FileName != "web-1.tar.gz" || !client.exported.InstanceOnly {
		t.Fatalf("exported = %+v", client.exported)
	}

	importResponse := handle(t, handler, http.MethodPost, "/exports/web-1.tar.gz/import", map[string]interface{}{"name": "web-1-restored"})
	if importResponse.StatusCode != http.StatusAccepted {
		t.Fatalf("import status = %d, want %d", importResponse.StatusCode, http.StatusAccepted)
	}
	if client.imported.FileName != "web-1.tar.gz" || client.imported.Name != "web-1-restored" {
		t.Fatalf("imported = %+v", client.imported)
	}
}

func TestRejectsNetworkCRUDAndPathTraversalExports(t *testing.T) {
	handler := NewHandler(Config{Project: "default"}, &fakeClient{})

	networkResponse := handle(t, handler, http.MethodPost, "/networks", map[string]string{"name": "ovn"})
	if networkResponse.StatusCode != http.StatusNotFound {
		t.Fatalf("network status = %d, want %d", networkResponse.StatusCode, http.StatusNotFound)
	}

	exportResponse := handle(t, handler, http.MethodPost, "/exports/../secret.tar.gz/import", map[string]string{"name": "bad"})
	if exportResponse.StatusCode != http.StatusBadRequest {
		t.Fatalf("path traversal status = %d, want %d", exportResponse.StatusCode, http.StatusBadRequest)
	}
}

func TestClientErrorMapsToHTTPError(t *testing.T) {
	handler := NewHandler(Config{Project: "default"}, &fakeClient{err: errors.New("incus down")})

	response := handle(t, handler, http.MethodGet, "/instances", nil)

	if response.StatusCode != http.StatusBadGateway {
		t.Fatalf("StatusCode = %d, want %d", response.StatusCode, http.StatusBadGateway)
	}
	body := decodeBody[map[string]interface{}](t, response)
	if body["error"] != "incus down" {
		t.Fatalf("error = %v, want incus down", body["error"])
	}
}

func handle(t *testing.T, handler *Handler, method, path string, payload interface{}) pluginipc.HTTPResponse {
	t.Helper()

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal payload: %v", err)
		}
	}

	response, err := handler.HandleHTTP(context.Background(), pluginipc.HTTPRequest{
		Method: method,
		Path:   path,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: body,
	})
	if err != nil {
		t.Fatalf("HandleHTTP() error = %v", err)
	}
	return response
}

func decodeBody[T any](t *testing.T, response pluginipc.HTTPResponse) T {
	t.Helper()

	var value T
	if err := json.Unmarshal(response.Body, &value); err != nil {
		t.Fatalf("decode body %s: %v", string(response.Body), err)
	}
	return value
}
