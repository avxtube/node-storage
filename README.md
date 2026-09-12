# node-storage

HTTP storage service สำหรับเสิร์ฟ media จาก storage record ในฐานข้อมูลปัจจุบัน รองรับ local disk และ S3-compatible storage

## การทำงาน

- อ่าน provider และตำแหน่งไฟล์จาก `storages`
  - local: `local.basePath`
  - S3: `s3.endpoint`, `region`, `bucket`, `prefix` และ encrypted credentials
- `/{mediaSlug}.mp4` เสิร์ฟ video หรือ audio source พร้อม HTTP Range
- `/{mediaSlug}.json` สร้าง manifest สำหรับ nginx-vod-module; S3 ใช้ `storage.originUrl + media.key` โดยตรง (origin ต้องรองรับ HTTP Range) ไม่เติม `s3.prefix` ซ้ำลงใน URL และต้องตั้ง `originUrl`
- `/{fileSlug}/{path}` เสิร์ฟ asset ภายใต้ `<fileId>/<path>`
- `/api/health` แสดงสถานะ `health.checkedAt`, capacity และ disk usage
- อัปเดต `storages.status`, `health` และ `capacity` ทุกหนึ่งนาที โดยไม่เขียนทับ `enabled`

Media ใช้ `fileId`, `storageId`, `quality`, `key`, `mime` ตาม `platform/packages/db/src/models/file-media.model.ts` โดยตรง ไม่มี path fallback, clone propagation หรือ cleanup จาก `media.deletedAt`

## Configuration

```env
DATABASE_URL=mongodb+srv://user:pass@cluster.mongodb.net/platform
STORAGE_ID=storage-uuid

# ต้องตั้งเมื่อ storage provider เป็น S3
STORAGE_ENCRYPTION_KEY=same-key-as-platform

PORT=8888
HOST=0.0.0.0
```

ตำแหน่ง local storage อ่านจาก `storages.local.basePath` จึงไม่ใช้ `STORAGE_PATH`

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/avxtube/node-storage/main/install.sh | sudo -E bash -s -- \
  --database-url "mongodb+srv://user:pass@cluster.mongodb.net/platform" \
  --storage-id "storage-uuid" \
  --storage-encryption-key "same-key-as-platform"
```

## Development

```bash
go test ./...
go vet ./...
go run ./cmd
```

MongoDB indexes จัดการจาก `platform/packages/db` เท่านั้น
