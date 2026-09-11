package storage

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin joins database-derived path components below base and rejects paths
// that would escape the configured storage directory.
func SafeJoin(base string, elements ...string) (string, error) {
	base = strings.TrimSpace(base)
	if base == "" {
		return "", fmt.Errorf("storage base path is empty")
	}

	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolve storage base path: %w", err)
	}
	targetAbs, err := filepath.Abs(filepath.Join(append([]string{baseAbs}, elements...)...))
	if err != nil {
		return "", fmt.Errorf("resolve storage target path: %w", err)
	}

	relative, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("compare storage paths: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", fmt.Errorf("path escapes storage directory")
	}

	return targetAbs, nil
}
