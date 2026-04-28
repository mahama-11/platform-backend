package models

import "time"

type StorageAsset struct {
	ID          string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode string     `gorm:"uniqueIndex:uidx_storage_asset_source;not null" json:"product_code"`
	Category    string     `gorm:"uniqueIndex:uidx_storage_asset_source;not null" json:"category"`
	SourceType  string     `gorm:"uniqueIndex:uidx_storage_asset_source;not null" json:"source_type"`
	SourceRef   string     `gorm:"uniqueIndex:uidx_storage_asset_source;not null" json:"source_ref"`
	StorageKey  string     `gorm:"uniqueIndex;not null" json:"storage_key"`
	FileName    string     `json:"file_name"`
	MimeType    string     `gorm:"index" json:"mime_type"`
	FileSize    int64      `json:"file_size"`
	Checksum    string     `gorm:"index" json:"checksum"`
	Title       string     `json:"title"`
	Description string     `gorm:"type:text" json:"description"`
	Tags        string     `gorm:"type:text" json:"tags"`
	Metadata    string     `gorm:"type:text" json:"metadata"`
	Status      string     `gorm:"index;not null" json:"status"`
	ImportedAt  *time.Time `gorm:"index" json:"imported_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}
