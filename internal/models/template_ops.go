package models

import "time"

type TemplateProjection struct {
	TemplateRef     string     `gorm:"type:varchar(128);primaryKey" json:"template_ref"`
	ProductCode     string     `gorm:"type:varchar(32);index;not null" json:"product_code"`
	TemplateID      string     `gorm:"type:varchar(128);index;not null" json:"template_id"`
	Slug            string     `gorm:"type:varchar(255);index" json:"slug"`
	Name            string     `gorm:"type:varchar(255);index;not null" json:"name"`
	Summary         string     `gorm:"type:text" json:"summary"`
	Status          string     `gorm:"type:varchar(32);index;not null;default:draft" json:"status"`
	PublishStatus   string     `gorm:"type:varchar(32);index;not null;default:draft" json:"publish_status"`
	Scope           string     `gorm:"type:varchar(32);index" json:"scope"`
	ManagedSource   string     `gorm:"type:varchar(32);index;not null;default:external_sync" json:"managed_source"`
	CoverAssetID    string     `gorm:"type:varchar(128);index" json:"cover_asset_id"`
	CoverAssetURL   string     `gorm:"type:text" json:"cover_asset_url"`
	RecommendScore  int        `gorm:"not null;default:0" json:"recommend_score"`
	PlatformsJSON   string     `gorm:"type:text" json:"platforms_json"`
	TagsJSON        string     `gorm:"type:text" json:"tags_json"`
	Series          string     `gorm:"type:varchar(128);index" json:"series"`
	CapabilityType  string     `gorm:"type:varchar(128);index" json:"capability_type"`
	Modality        string     `gorm:"type:varchar(64);index" json:"modality"`
	RawJSON         string     `gorm:"type:text" json:"raw_json"`
	DetailJSON      string     `gorm:"type:text" json:"detail_json"`
	SourceUpdatedAt time.Time  `gorm:"index" json:"source_updated_at"`
	LastSyncedAt    time.Time  `gorm:"index" json:"last_synced_at"`
	PublishedAt     *time.Time `json:"published_at"`
	CreatedAt       time.Time  `gorm:"index;autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}
