package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"time"

	"storage-node/internal/config"
	"storage-node/internal/core/enums"
	"storage-node/internal/db/models"
	"storage-node/internal/storage"

	"go.mongodb.org/mongo-driver/bson"
)

type Handler struct {
	StorageID string
}

type VODClip struct {
	Type       string `json:"type"`
	SourceType string `json:"sourceType"`
	Path       string `json:"path"`
}

type VODSequence struct {
	Clips []VODClip `json:"clips"`
}

type VODManifest struct {
	Sequences []VODSequence `json:"sequences"`
}

func NewHandler(cfg Handler) *Handler { return &cfg }

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
		HandleNotFound(w, r)
		return
	}
	requestPath := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(requestPath, "/", 2)
	switch {
	case len(parts) == 1 && strings.HasSuffix(requestPath, ".mp4"):
		h.ServeVideo(w, r, strings.TrimSuffix(requestPath, ".mp4"))
	case len(parts) == 1 && strings.HasSuffix(requestPath, ".json"):
		h.ServeVODManifest(w, r, strings.TrimSuffix(requestPath, ".json"))
	case len(parts) == 2:
		h.ServeFile(w, r, parts[0], parts[1])
	default:
		HandleNotFound(w, r)
	}
}

func (h *Handler) findVideoMedia(r *http.Request, slug string) (*models.Media, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	return models.MediaModel.FindOne(ctx, bson.M{
		"type": enums.MediaTypeVideo, "slug": slug, "storageId": h.StorageID,
		"quality": bson.M{"$in": []interface{}{enums.ResolutionOriginal, enums.Resolution1080, enums.Resolution720, enums.Resolution480, enums.Resolution360, nil}},
	})
}

func (h *Handler) findVODMedia(r *http.Request, slug string) (*models.Media, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	return models.MediaModel.FindOne(ctx, bson.M{
		"slug": slug, "storageId": h.StorageID, "type": bson.M{"$in": []string{enums.MediaTypeVideo, enums.MediaTypeAudio}},
	})
}

func (h *Handler) findStorage(r *http.Request) (*models.Storage, error) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	storageConfig, err := models.StorageModel.FindByID(ctx, h.StorageID)
	if err != nil {
		return nil, err
	}
	if !storageConfig.IsOnline() {
		return nil, fmt.Errorf("storage is disabled or offline")
	}
	return storageConfig, nil
}

func resolvedMediaObjectPath(media *models.Media) (string, error) {
	if media == nil {
		return "", fmt.Errorf("media is nil")
	}
	raw := strings.ReplaceAll(strings.TrimSpace(media.ObjectPath()), "\\", "/")
	cleaned := pathpkg.Clean(raw)
	if raw == "" || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.HasPrefix(raw, "/") {
		return "", fmt.Errorf("media key is unsafe")
	}
	return cleaned, nil
}

func buildLocalMediaPath(basePath string, media *models.Media) (string, error) {
	key, err := resolvedMediaObjectPath(media)
	if err != nil {
		return "", err
	}
	return storage.SafeJoin(basePath, filepath.FromSlash(key))
}

func buildRemoteVODPath(originURL string, storageConfig *models.Storage, media *models.Media) (string, error) {
	originURL = strings.TrimSpace(originURL)
	if originURL == "" {
		return "", fmt.Errorf("origin URL is empty")
	}
	if !strings.Contains(originURL, "://") {
		originURL = "https://" + originURL
	}
	origin, err := url.Parse(originURL)
	if err != nil || (origin.Scheme != "http" && origin.Scheme != "https") || origin.Host == "" || origin.User != nil || origin.RawQuery != "" || origin.Fragment != "" {
		return "", fmt.Errorf("invalid HTTP origin URL")
	}
	key, err := resolvedMediaObjectPath(media)
	if err != nil {
		return "", err
	}
	key = storage.PhysicalObjectKey(storageConfig, key)
	parts := []string{"", origin.Scheme, origin.Host}
	for _, segment := range strings.Split(strings.Trim(origin.Path, "/")+"/"+key, "/") {
		if segment != "" {
			parts = append(parts, url.PathEscape(segment))
		}
	}
	return strings.Join(parts, "/"), nil
}

func buildVODClip(storageConfig *models.Storage, media *models.Media) (VODClip, error) {
	switch storageConfig.Provider {
	case enums.StorageTypeLocal:
		if storageConfig.Local == nil {
			return VODClip{}, fmt.Errorf("local storage has no config")
		}
		filePath, err := buildLocalMediaPath(storageConfig.Local.BasePath, media)
		return VODClip{Type: "source", SourceType: "file", Path: filePath}, err
	case enums.StorageTypeS3:
		originURL := storageConfig.OriginURL
		if originURL == nil || strings.TrimSpace(*originURL) == "" {
			originURL = storageConfig.PublicURL
		}
		if originURL == nil || strings.TrimSpace(*originURL) == "" {
			return VODClip{}, fmt.Errorf("S3 storage has no originUrl or publicUrl")
		}
		remotePath, err := buildRemoteVODPath(*originURL, storageConfig, media)
		return VODClip{Type: "source", SourceType: "http", Path: remotePath}, err
	default:
		return VODClip{}, fmt.Errorf("unsupported storage provider %q", storageConfig.Provider)
	}
}

func (h *Handler) ServeVideo(w http.ResponseWriter, r *http.Request, slug string) {
	media, err := h.findVideoMedia(r, slug)
	if err != nil {
		HandleNotFound(w, r)
		return
	}
	storageConfig, err := h.findStorage(r)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	if storageConfig.Provider == enums.StorageTypeS3 {
		h.serveS3Media(w, r, storageConfig, media)
		return
	}
	if storageConfig.Local == nil {
		handleStorageError(w, fmt.Errorf("local storage has no config"))
		return
	}
	filePath, err := buildLocalMediaPath(storageConfig.Local.BasePath, media)
	if err != nil {
		HandleNotFound(w, r)
		return
	}
	if info, err := os.Stat(filePath); err != nil || info.IsDir() {
		HandleNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", media.Mime)
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeFile(w, r, filePath)
}

func (h *Handler) ServeVODManifest(w http.ResponseWriter, r *http.Request, slug string) {
	media, err := h.findVODMedia(r, slug)
	if err != nil {
		HandleNotFound(w, r)
		return
	}
	storageConfig, err := h.findStorage(r)
	if err != nil {
		handleStorageError(w, err)
		return
	}
	clip, err := buildVODClip(storageConfig, media)
	if err != nil {
		log.Printf("build VOD clip: %v", err)
		HandleNotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(VODManifest{Sequences: []VODSequence{{Clips: []VODClip{clip}}}})
}

func (h *Handler) serveS3Media(w http.ResponseWriter, r *http.Request, storageConfig *models.Storage, media *models.Media) {
	if err := storage.PrepareStorageCredentials(storageConfig, config.AppConfig.StorageEncryptionKey); err != nil {
		handleStorageError(w, err)
		return
	}
	key, err := resolvedMediaObjectPath(media)
	if err != nil {
		HandleNotFound(w, r)
		return
	}
	result, err := storage.GetS3Object(r.Context(), storageConfig, key, r.Header.Get("Range"))
	if err != nil {
		log.Printf("get S3 object %s: %v", key, err)
		HandleNotFound(w, r)
		return
	}
	defer result.Body.Close()
	copyS3Response(w, r, result, media.Mime)
}

func handleStorageError(w http.ResponseWriter, err error) {
	log.Printf("storage unavailable: %v", err)
	http.Error(w, "storage unavailable", http.StatusServiceUnavailable)
}
