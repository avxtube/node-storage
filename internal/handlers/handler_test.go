package handlers

import (
	"path/filepath"
	"testing"

	"storage-node/internal/core/enums"
	"storage-node/internal/db/models"
)

func TestBuildRemoteVODPathUsesMediaKey(t *testing.T) {
	origin := "https://origin.example/videos/"
	media := &models.Media{Key: "file-id/file_1080.mp4"}
	got, err := buildRemoteVODPath(origin, media)
	if err != nil {
		t.Fatal(err)
	}
	if want := "/https/origin.example/videos/file-id/file_1080.mp4"; got != want {
		t.Fatalf("remote VOD path = %q, want %q", got, want)
	}
}

func TestBuildRemoteVODPathRejectsUnsafeKey(t *testing.T) {
	if _, err := buildRemoteVODPath("https://origin.example", &models.Media{Key: "../secret"}); err == nil {
		t.Fatal("expected unsafe key to be rejected")
	}
}

func TestBuildLocalMediaPathUsesMediaKey(t *testing.T) {
	base := t.TempDir()
	media := &models.Media{Key: "file-id/audio_1.m4a"}
	got, err := buildLocalMediaPath(base, media)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(base, "file-id", "audio_1.m4a"); got != want {
		t.Fatalf("local media path = %q, want %q", got, want)
	}
}

func TestBuildLocalMediaPathRejectsTraversal(t *testing.T) {
	if _, err := buildLocalMediaPath(t.TempDir(), &models.Media{Key: "../outside.mp4"}); err == nil {
		t.Fatal("expected traversal key to be rejected")
	}
}

func TestBuildVODClipSelectsLocalFile(t *testing.T) {
	base := t.TempDir()
	storageConfig := &models.Storage{Provider: enums.StorageTypeLocal, Local: &models.StorageLocalConfig{BasePath: base}}
	media := &models.Media{Key: "file-id/file_720.mp4"}
	got, err := buildVODClip(storageConfig, media)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceType != "file" || got.Path != filepath.Join(base, "file-id", "file_720.mp4") {
		t.Fatalf("local clip = %#v", got)
	}
}

func TestBuildVODClipUsesS3Origin(t *testing.T) {
	origin := "https://origin.example/library/"
	public := "https://public.example"
	storageConfig := &models.Storage{Provider: enums.StorageTypeS3, OriginURL: &origin, PublicURL: &public, S3: &models.StorageS3Config{Prefix: "library"}}
	media := &models.Media{Slug: "audio-media", Key: "file-id/audio_1.m4a"}
	got, err := buildVODClip(storageConfig, media)
	if err != nil {
		t.Fatal(err)
	}
	if got.SourceType != "http" || got.Path != "/https/origin.example/library/file-id/audio_1.m4a" {
		t.Fatalf("S3 clip = %#v", got)
	}
}

func TestBuildVODClipRequiresOrigin(t *testing.T) {
	public := "https://public.example"
	for _, origin := range []*string{nil, new(string)} {
		cfg := &models.Storage{Provider: enums.StorageTypeS3, OriginURL: origin, PublicURL: &public}
		if _, err := buildVODClip(cfg, &models.Media{Key: "file/video.mp4"}); err == nil {
			t.Fatal("expected missing originUrl to fail instead of using publicUrl")
		}
	}
}
