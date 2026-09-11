package handlers

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"storage-node/internal/core/enums"
	"storage-node/internal/db/models"
	"storage-node/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
)

func (h *Handler) ServeFile(w http.ResponseWriter, r *http.Request, slug, subPath string) {
	cleanSubPath := path.Clean(strings.ReplaceAll(subPath, "\\", "/"))
	if cleanSubPath == "." || cleanSubPath == ".." || strings.HasPrefix(cleanSubPath, "../") || strings.HasPrefix(subPath, "/") {
		HandleNotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	file, err := models.FileModel.FindOne(ctx, bson.M{"slug": slug})
	if err != nil || file.IsTrashed() || file.IsDeleted() {
		HandleNotFound(w, r)
		return
	}
	storageConfig, err := h.findStorage(r)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	key := path.Join(file.ID, cleanSubPath)
	media := &models.Media{Key: key, Mime: "application/octet-stream"}
	if registered, findErr := models.MediaModel.FindOne(ctx, bson.M{
		"storageId": h.StorageID, "fileId": file.ID, "key": key,
	}); findErr == nil {
		media = registered
	}
	if storageConfig.Provider == enums.StorageTypeS3 {
		h.serveS3Media(w, r, storageConfig, media)
		return
	}
	if storageConfig.Local == nil {
		handleStorageError(w, fmt.Errorf("local storage has no config"))
		return
	}
	filePath, err := storage.SafeJoin(storageConfig.Local.BasePath, filepath.FromSlash(key))
	if err != nil {
		HandleNotFound(w, r)
		return
	}
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		HandleNotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, filePath)
}
