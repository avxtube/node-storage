package models

import "strings"

// ObjectPath returns the storage-relative key persisted by the transfer flow.
func (m *Media) ObjectPath() string {
	if m == nil {
		return ""
	}
	return strings.TrimLeft(strings.ReplaceAll(strings.TrimSpace(m.Key), "\\", "/"), "/")
}

func (f *File) IsTrashed() bool {
	return f.Metadata != nil && f.Metadata.TrashedAt != nil
}

func (f *File) IsDeleted() bool {
	return f.Metadata != nil && f.Metadata.DeletedAt != nil
}
