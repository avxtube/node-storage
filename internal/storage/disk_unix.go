//go:build linux || darwin
// +build linux darwin

package storage

import (
	"fmt"
	"syscall"
)

// getDiskUsageOS returns disk usage for Linux/Darwin
func getDiskUsageOS(path string) (*DiskUsage, error) {
	var stat syscall.Statfs_t
	err := syscall.Statfs(path, &stat)
	if err != nil {
		return nil, fmt.Errorf("failed to get disk stats: %w", err)
	}

	// Bavail ไม่ใช่ Bfree — ext4 กัน reserved-blocks ไว้ให้ root (default 5%)
	// ซึ่ง process ที่เขียนไฟล์จริง (nginx/videohide) แตะไม่ได้ ใช้ Bfree จะ
	// รายงานว่างเกินจริงเท่าขนาด reserved (37TB → เพี้ยน ~1.8TB / ~5%) แล้ว
	// cutoff maxPercent ของ transfer enqueuer จะไม่ทำงานจนดิสก์เต็มจริงไปแล้ว
	// ค่านี้ตรงกับ Avail/Use% ของ df และกับที่ worker-transfer คำนวณ
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free

	percentage := 0.0
	if total > 0 {
		percentage = float64(used) / float64(total) * 100
	}

	return &DiskUsage{
		Total:      total,
		Used:       used,
		Free:       free,
		Percentage: percentage,
	}, nil
}
