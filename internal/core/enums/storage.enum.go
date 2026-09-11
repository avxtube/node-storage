package enums

// ─── Storage Types ───────────────────────────────────────────────────

const (
	StorageTypeLocal = "local"
	StorageTypeS3    = "s3"
)

// ─── Storage Statuses ────────────────────────────────────────────────

const (
	StorageStatusUnknown = "unknown"
	StorageStatusOnline  = "online"
	StorageStatusOffline = "offline"
	StorageStatusError   = "error"
)

const (
	StoragePurposeUpload  = "upload"
	StoragePurposeStorage = "storage"
	StoragePurposeTemp    = "temp"
)
