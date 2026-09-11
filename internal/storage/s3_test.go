package storage

import (
	"testing"

	"storage-node/internal/db/models"
)

func TestPhysicalObjectKeyAddsPrefixOnce(t *testing.T) {
	storageConfig := &models.Storage{S3: &models.StorageS3Config{Prefix: "media/root"}}
	if got := PhysicalObjectKey(storageConfig, "file-id/file.mp4"); got != "media/root/file-id/file.mp4" {
		t.Fatalf("prefixed key = %q", got)
	}
	if got := PhysicalObjectKey(storageConfig, "media/root/file-id/file.mp4"); got != "media/root/file-id/file.mp4" {
		t.Fatalf("already-prefixed key = %q", got)
	}
}

func TestNormalizeEndpoint(t *testing.T) {
	if got := normalizeEndpoint("objects.example/bucket/", "bucket"); got != "https://objects.example" {
		t.Fatalf("endpoint = %q", got)
	}
}
