package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"storage-node/internal/config"
	"storage-node/internal/core/enums"
	"storage-node/internal/db/database"
	"storage-node/internal/db/models"
	"storage-node/internal/handlers"
	"storage-node/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
)

var version = "dev"

const heartbeatInterval = time.Minute

func main() {
	config.Load()
	if config.AppConfig.StorageId == "" {
		log.Println("STORAGE_ID environment variable is required")
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}
	log.Printf("Starting Storage Node %s [storage=%s]", version, config.AppConfig.StorageId)

	if err := database.Connect(); err != nil {
		log.Printf("Failed to connect to MongoDB: %v", err)
		time.Sleep(5 * time.Second)
		os.Exit(1)
	}
	defer database.Disconnect()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := updateStorageHealth(ctx); err != nil {
		log.Printf("Initial storage health check failed: %v", err)
	}
	go heartbeat(ctx)

	h := handlers.NewHandler(handlers.Handler{StorageID: config.AppConfig.StorageId})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", h.Health)
	mux.HandleFunc("/", h.Home)

	server := &http.Server{
		Addr: net.JoinHostPort(config.AppConfig.Host, config.AppConfig.Port), Handler: withCORS(mux),
		ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20,
	}
	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Storage server listening on http://%s", server.Addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		log.Printf("HTTP server stopped unexpectedly: %v", err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	_ = markStorageStatus(shutdownCtx, enums.StorageStatusOffline, nil)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Range")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func heartbeat(ctx context.Context) {
	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := updateStorageHealth(ctx); err != nil && ctx.Err() == nil {
				log.Printf("Storage health check failed: %v", err)
			}
		}
	}
}

func updateStorageHealth(ctx context.Context) error {
	storageConfig, err := models.StorageModel.FindByID(ctx, config.AppConfig.StorageId)
	if err != nil {
		return err
	}
	if storageConfig.DeletedAt != nil {
		return fmt.Errorf("storage %s is deleted", storageConfig.ID)
	}
	started := time.Now()
	switch storageConfig.Provider {
	case enums.StorageTypeLocal:
		if storageConfig.Local == nil || storageConfig.Local.BasePath == "" {
			return markHealthError(ctx, storageConfig.ID, "local.basePath is required")
		}
		usage, err := storage.GetDiskUsage(storageConfig.Local.BasePath)
		if err != nil {
			_ = markHealthError(ctx, storageConfig.ID, err.Error())
			return err
		}
		latency := float64(time.Since(started).Microseconds()) / 1000
		now := time.Now()
		_, err = models.StorageModel.Col().UpdateOne(ctx, bson.M{"_id": storageConfig.ID}, bson.M{"$set": bson.M{
			"status":    enums.StorageStatusOnline,
			"health":    bson.M{"checkedAt": now, "latencyMs": latency},
			"capacity":  bson.M{"totalBytes": int64(usage.Total), "usedBytes": int64(usage.Used), "freeBytes": int64(usage.Free)},
			"updatedAt": now,
		}})
		return err
	case enums.StorageTypeS3:
		if err := storage.PrepareStorageCredentials(storageConfig, config.AppConfig.StorageEncryptionKey); err != nil {
			_ = markHealthError(ctx, storageConfig.ID, err.Error())
			return err
		}
		s3Config := storage.S3Config{
			Region: storageConfig.S3.Region, Bucket: storageConfig.S3.Bucket,
			AccessKeyID: storageConfig.S3.AccessKeyID, SecretAccessKey: storageConfig.S3.SecretAccessKey,
			ForcePathStyle: storageConfig.S3.ForcePathStyle,
		}
		if storageConfig.S3.Endpoint != nil {
			s3Config.Endpoint = *storageConfig.S3.Endpoint
		}
		checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		client, err := storage.NewS3Client(checkCtx, s3Config)
		if err == nil {
			err = storage.CheckS3Bucket(checkCtx, client, s3Config.Bucket)
		}
		if err != nil {
			_ = markHealthError(ctx, storageConfig.ID, err.Error())
			return err
		}
		latency := float64(time.Since(started).Microseconds()) / 1000
		return markStorageStatus(ctx, enums.StorageStatusOnline, &latency)
	default:
		return markHealthError(ctx, storageConfig.ID, "unsupported storage provider")
	}
}

func markHealthError(ctx context.Context, storageID, message string) error {
	now := time.Now()
	_, err := models.StorageModel.Col().UpdateOne(ctx, bson.M{"_id": storageID}, bson.M{"$set": bson.M{
		"status": enums.StorageStatusError, "health": bson.M{"checkedAt": now, "message": message}, "updatedAt": now,
	}})
	if err != nil {
		return err
	}
	return fmt.Errorf("%s", message)
}

func markStorageStatus(ctx context.Context, status string, latencyMS *float64) error {
	now := time.Now()
	health := bson.M{"checkedAt": now}
	if latencyMS != nil {
		health["latencyMs"] = *latencyMS
	}
	result, err := models.StorageModel.Col().UpdateOne(ctx, bson.M{"_id": config.AppConfig.StorageId}, bson.M{"$set": bson.M{
		"status": status, "health": health, "updatedAt": now,
	}})
	if err != nil {
		return err
	}
	if result.MatchedCount == 0 {
		return fmt.Errorf("storage not found: %s", config.AppConfig.StorageId)
	}
	return nil
}
