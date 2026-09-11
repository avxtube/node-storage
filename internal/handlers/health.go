package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"storage-node/internal/core/enums"
	"storage-node/internal/db/models"
	"storage-node/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
)

const healthHeartbeatMaxAge = 3 * time.Minute

type HealthResponse struct {
	Status        string                  `json:"status"`
	StorageID     string                  `json:"storageId"`
	Provider      string                  `json:"provider,omitempty"`
	StorageStatus string                  `json:"storageStatus,omitempty"`
	CheckedAt     *time.Time              `json:"checkedAt,omitempty"`
	Uptime        string                  `json:"uptime"`
	Capacity      *models.StorageCapacity `json:"capacity,omitempty"`
	Disk          *DiskHealth             `json:"disk,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

type DiskHealth struct {
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Free       int64   `json:"free"`
	Percentage float64 `json:"percentage"`
}

var startedAt = time.Now()

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	resp := HealthResponse{Status: "error", StorageID: h.StorageID, Uptime: time.Since(startedAt).Round(time.Second).String()}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	storageConfig, err := models.StorageModel.FindOne(ctx, bson.M{"_id": h.StorageID})
	if err != nil {
		resp.Error = fmt.Sprintf("storage configuration unavailable: %v", err)
		writeHealth(w, http.StatusServiceUnavailable, resp)
		return
	}
	resp.Provider = storageConfig.Provider
	resp.StorageStatus = storageConfig.Status
	resp.Capacity = storageConfig.Capacity
	if storageConfig.Health != nil {
		resp.CheckedAt = storageConfig.Health.CheckedAt
	}
	if !storageConfig.IsOnline() {
		resp.Error = "storage backend is disabled or offline"
		writeHealth(w, http.StatusServiceUnavailable, resp)
		return
	}
	if resp.CheckedAt == nil || time.Since(*resp.CheckedAt) > healthHeartbeatMaxAge {
		resp.Error = "storage health check is stale"
		writeHealth(w, http.StatusServiceUnavailable, resp)
		return
	}
	if storageConfig.Provider == enums.StorageTypeLocal {
		if storageConfig.Local == nil {
			resp.Error = "local storage configuration is missing"
			writeHealth(w, http.StatusServiceUnavailable, resp)
			return
		}
		usage, err := storage.GetDiskUsage(storageConfig.Local.BasePath)
		if err != nil {
			resp.Error = fmt.Sprintf("local disk unavailable: %v", err)
			writeHealth(w, http.StatusServiceUnavailable, resp)
			return
		}
		resp.Disk = &DiskHealth{Total: int64(usage.Total), Used: int64(usage.Used), Free: int64(usage.Free), Percentage: usage.Percentage}
	}
	resp.Status = "ok"
	writeHealth(w, http.StatusOK, resp)
}

func writeHealth(w http.ResponseWriter, statusCode int, resp HealthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(resp)
}
