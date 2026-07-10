package incusplugin

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/CUTe-CCNL/host-agent/pkg/pluginipc"
)

type Handler struct {
	config Config
	client Client
}

func NewHandler(config Config, client Client) *Handler {
	return &Handler{config: config, client: client}
}

func (h *Handler) Health(ctx context.Context) error {
	_, err := h.client.Ping(ctx)
	return err
}

func (h *Handler) HandleHTTP(ctx context.Context, req pluginipc.HTTPRequest) (pluginipc.HTTPResponse, error) {
	if hasPathTraversal(req.Path) {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "invalid path"})
	}

	requestPath := normalizePath(req.Path)
	segments := splitPath(requestPath)

	if req.Method == http.MethodGet && requestPath == "/status" {
		return h.status(ctx)
	}

	if len(segments) > 0 {
		switch segments[0] {
		case "instances":
			return h.handleInstances(ctx, req, segments)
		case "exports":
			return h.handleExports(ctx, req, segments)
		}
	}

	return jsonResponse(http.StatusNotFound, map[string]string{"error": "route not found"})
}

func (h *Handler) status(ctx context.Context) (pluginipc.HTTPResponse, error) {
	info, err := h.client.Ping(ctx)
	if err != nil {
		return errorResponse(err), nil
	}
	if info.Project == "" {
		info.Project = h.config.Project
	}

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"status":      "ok",
		"server_name": info.ServerName,
		"version":     info.Version,
		"project":     info.Project,
	})
}

func (h *Handler) handleInstances(ctx context.Context, req pluginipc.HTTPRequest, segments []string) (pluginipc.HTTPResponse, error) {
	switch {
	case len(segments) == 1 && req.Method == http.MethodGet:
		query, _ := url.ParseQuery(req.RawQuery)
		instances, err := h.client.ListInstances(ctx, query.Get("type"))
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusOK, map[string]interface{}{"instances": instances})

	case len(segments) == 1 && req.Method == http.MethodPost:
		createReq, err := decodeCreateInstance(req.Body, h.config.StoragePool)
		if err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result, err := h.client.CreateInstance(ctx, createReq)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 2 && req.Method == http.MethodGet:
		instance, err := h.client.GetInstance(ctx, segments[1])
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusOK, instance)

	case len(segments) == 2 && req.Method == http.MethodDelete:
		query, _ := url.ParseQuery(req.RawQuery)
		result, err := h.client.DeleteInstance(ctx, segments[1], parseBool(query.Get("force")))
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 3 && req.Method == http.MethodPost && segments[2] == "start":
		result, err := h.client.StartInstance(ctx, segments[1])
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 3 && req.Method == http.MethodPost && segments[2] == "stop":
		force := forceFromBodyOrQuery(req)
		result, err := h.client.StopInstance(ctx, segments[1], force)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 3 && req.Method == http.MethodPost && segments[2] == "restart":
		force := forceFromBodyOrQuery(req)
		result, err := h.client.RestartInstance(ctx, segments[1], force)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) >= 3 && segments[2] == "snapshots":
		return h.handleSnapshots(ctx, req, segments)

	case len(segments) == 3 && req.Method == http.MethodPost && segments[2] == "exports":
		exportReq, err := decodeExportInstance(segments[1], req.Body)
		if err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		metadata, err := h.client.ExportInstance(ctx, exportReq)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, metadata)
	}

	return jsonResponse(http.StatusNotFound, map[string]string{"error": "route not found"})
}

func (h *Handler) handleSnapshots(ctx context.Context, req pluginipc.HTTPRequest, segments []string) (pluginipc.HTTPResponse, error) {
	instanceName := segments[1]

	switch {
	case len(segments) == 3 && req.Method == http.MethodGet:
		snapshots, err := h.client.ListSnapshots(ctx, instanceName)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusOK, map[string]interface{}{"snapshots": snapshots})

	case len(segments) == 3 && req.Method == http.MethodPost:
		snapshotReq, err := decodeCreateSnapshot(instanceName, req.Body)
		if err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result, err := h.client.CreateSnapshot(ctx, snapshotReq)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 5 && req.Method == http.MethodPost && segments[4] == "restore":
		restoreReq, err := decodeRestoreSnapshot(instanceName, segments[3], req.Body)
		if err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result, err := h.client.RestoreSnapshot(ctx, restoreReq)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 4 && req.Method == http.MethodDelete:
		result, err := h.client.DeleteSnapshot(ctx, DeleteSnapshotRequest{
			InstanceName: instanceName,
			SnapshotName: segments[3],
		})
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)
	}

	return jsonResponse(http.StatusNotFound, map[string]string{"error": "route not found"})
}

func (h *Handler) handleExports(ctx context.Context, req pluginipc.HTTPRequest, segments []string) (pluginipc.HTTPResponse, error) {
	switch {
	case len(segments) == 1 && req.Method == http.MethodGet:
		exports, err := h.client.ListExports(ctx)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusOK, map[string]interface{}{"exports": exports})

	case len(segments) == 3 && req.Method == http.MethodPost && segments[2] == "import":
		importReq, err := decodeImportExport(segments[1], req.Body)
		if err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		result, err := h.client.ImportExport(ctx, importReq)
		if err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusAccepted, result)

	case len(segments) == 2 && req.Method == http.MethodDelete:
		if err := validateFileName(segments[1]); err != nil {
			return jsonResponse(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		if err := h.client.DeleteExport(ctx, segments[1]); err != nil {
			return errorResponse(err), nil
		}
		return jsonResponse(http.StatusOK, map[string]string{"status": "deleted"})
	}

	return jsonResponse(http.StatusNotFound, map[string]string{"error": "route not found"})
}

func jsonResponse(statusCode int, value interface{}) (pluginipc.HTTPResponse, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return pluginipc.HTTPResponse{}, err
	}
	return pluginipc.HTTPResponse{
		StatusCode: statusCode,
		Headers: map[string][]string{
			"Content-Type": {"application/json"},
		},
		Body: body,
	}, nil
}

func errorResponse(err error) pluginipc.HTTPResponse {
	statusCode := http.StatusBadGateway
	if errors.Is(err, context.DeadlineExceeded) {
		statusCode = http.StatusGatewayTimeout
	}
	response, responseErr := jsonResponse(statusCode, map[string]string{"error": err.Error()})
	if responseErr != nil {
		return pluginipc.HTTPResponse{
			StatusCode: http.StatusInternalServerError,
			Headers:    map[string][]string{"Content-Type": {"application/json"}},
			Body:       []byte(`{"error":"failed to encode error"}`),
		}
	}
	return response
}

type createInstancePayload struct {
	Name     string                       `json:"name"`
	Type     string                       `json:"type"`
	Image    string                       `json:"image"`
	Profiles []string                     `json:"profiles"`
	Network  string                       `json:"network"`
	CPU      int                          `json:"cpu"`
	Memory   string                       `json:"memory"`
	Disk     string                       `json:"disk"`
	Start    bool                         `json:"start"`
	Config   map[string]string            `json:"config"`
	Devices  map[string]map[string]string `json:"devices"`
}

func decodeCreateInstance(body []byte, storagePool string) (CreateInstanceRequest, error) {
	var payload createInstancePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return CreateInstanceRequest{}, err
	}
	if payload.Name == "" {
		return CreateInstanceRequest{}, errors.New("name is required")
	}
	if payload.Image == "" {
		return CreateInstanceRequest{}, errors.New("image is required")
	}
	if payload.Type == "" {
		payload.Type = "container"
	}
	if payload.Type != "container" && payload.Type != "virtual-machine" {
		return CreateInstanceRequest{}, errors.New("type must be container or virtual-machine")
	}

	config := cloneStringMap(payload.Config)
	if payload.CPU > 0 {
		config["limits.cpu"] = strconv.Itoa(payload.CPU)
	}
	if payload.Memory != "" {
		config["limits.memory"] = payload.Memory
	}

	devices := cloneNestedStringMap(payload.Devices)
	if payload.Disk != "" {
		if storagePool == "" {
			storagePool = defaultStoragePool
		}
		if devices["root"] == nil {
			devices["root"] = map[string]string{"type": "disk", "path": "/", "pool": storagePool}
		}
		devices["root"]["type"] = "disk"
		devices["root"]["path"] = "/"
		if devices["root"]["pool"] == "" {
			devices["root"]["pool"] = storagePool
		}
		devices["root"]["size"] = payload.Disk
	}
	if payload.Network != "" {
		if devices["eth0"] == nil {
			devices["eth0"] = map[string]string{"type": "nic"}
		}
		devices["eth0"]["type"] = "nic"
		devices["eth0"]["network"] = payload.Network
	}

	return CreateInstanceRequest{
		Name:     payload.Name,
		Type:     payload.Type,
		Image:    payload.Image,
		Profiles: payload.Profiles,
		Network:  payload.Network,
		Start:    payload.Start,
		Config:   config,
		Devices:  devices,
	}, nil
}

func decodeCreateSnapshot(instanceName string, body []byte) (CreateSnapshotRequest, error) {
	var payload struct {
		Name     string `json:"name"`
		Stateful bool   `json:"stateful"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return CreateSnapshotRequest{}, err
		}
	}
	if payload.Name == "" {
		return CreateSnapshotRequest{}, errors.New("name is required")
	}
	return CreateSnapshotRequest{InstanceName: instanceName, Name: payload.Name, Stateful: payload.Stateful}, nil
}

func decodeRestoreSnapshot(instanceName, snapshotName string, body []byte) (RestoreSnapshotRequest, error) {
	var payload struct {
		Stateful bool `json:"stateful"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return RestoreSnapshotRequest{}, err
		}
	}
	return RestoreSnapshotRequest{InstanceName: instanceName, SnapshotName: snapshotName, Stateful: payload.Stateful}, nil
}

func decodeExportInstance(instanceName string, body []byte) (ExportInstanceRequest, error) {
	var payload struct {
		FileName      string `json:"file_name"`
		InstanceOnly  bool   `json:"instance_only"`
		Optimized     bool   `json:"optimized"`
		Compression   string `json:"compression"`
		IncludeImages bool   `json:"include_images"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return ExportInstanceRequest{}, err
		}
	}
	if payload.FileName != "" {
		if err := validateFileName(payload.FileName); err != nil {
			return ExportInstanceRequest{}, err
		}
	}
	return ExportInstanceRequest{
		InstanceName:  instanceName,
		FileName:      payload.FileName,
		InstanceOnly:  payload.InstanceOnly,
		Optimized:     payload.Optimized,
		Compression:   payload.Compression,
		IncludeImages: payload.IncludeImages,
	}, nil
}

func decodeImportExport(fileName string, body []byte) (ImportExportRequest, error) {
	if err := validateFileName(fileName); err != nil {
		return ImportExportRequest{}, err
	}
	var payload struct {
		Name string `json:"name"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return ImportExportRequest{}, err
		}
	}
	return ImportExportRequest{FileName: fileName, Name: payload.Name}, nil
}

func forceFromBodyOrQuery(req pluginipc.HTTPRequest) bool {
	query, _ := url.ParseQuery(req.RawQuery)
	if parseBool(query.Get("force")) {
		return true
	}
	var payload struct {
		Force bool `json:"force"`
	}
	if len(req.Body) > 0 {
		_ = json.Unmarshal(req.Body, &payload)
	}
	return payload.Force
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(value)
	return parsed
}

func normalizePath(value string) string {
	if value == "" {
		return "/"
	}
	if !strings.HasPrefix(value, "/") {
		value = "/" + value
	}
	return path.Clean(value)
}

func splitPath(value string) []string {
	value = strings.Trim(normalizePath(value), "/")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func hasPathTraversal(value string) bool {
	for _, segment := range strings.Split(value, "/") {
		if segment == ".." {
			return true
		}
	}
	return false
}

func validateFileName(fileName string) error {
	if fileName == "" {
		return errors.New("file_name is required")
	}
	if fileName != path.Base(fileName) || strings.Contains(fileName, "..") {
		return errors.New("invalid file_name")
	}
	return nil
}

func cloneStringMap(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func cloneNestedStringMap(source map[string]map[string]string) map[string]map[string]string {
	target := make(map[string]map[string]string, len(source))
	for key, values := range source {
		target[key] = cloneStringMap(values)
	}
	return target
}
