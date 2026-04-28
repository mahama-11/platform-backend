package models

import "time"

type ResourceReservation struct {
	ID                 string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ResourceType       string     `gorm:"index;not null" json:"resource_type"`
	BillingSubjectType string     `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string     `gorm:"index;not null" json:"billing_subject_id"`
	BillableItemCode   string     `gorm:"index" json:"billable_item_code"`
	ReservationKey     *string    `gorm:"type:varchar(128);uniqueIndex" json:"reservation_key,omitempty"`
	FinalizationID     *string    `gorm:"type:varchar(128);index" json:"finalization_id,omitempty"`
	Units              int64      `json:"units"`
	Status             string     `gorm:"index;not null" json:"status"`
	ReferenceID        string     `gorm:"index" json:"reference_id"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	ExpiresAt          *time.Time `json:"expires_at"`
	CommittedAt        *time.Time `json:"committed_at"`
	ReleasedAt         *time.Time `json:"released_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}
