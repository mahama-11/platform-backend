package models

import "time"

type RuntimeProviderDefinition struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Code          string    `gorm:"uniqueIndex;not null" json:"code"`
	Name          string    `gorm:"not null" json:"name"`
	ProviderType  string    `gorm:"index;not null" json:"provider_type"`
	Mode          string    `gorm:"index;not null" json:"mode"`
	CredentialRef string    `json:"credential_ref"`
	Capabilities  string    `gorm:"type:text" json:"capabilities"`
	Status        string    `gorm:"index;not null" json:"status"`
	Metadata      string    `gorm:"type:text" json:"metadata"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type RuntimeProductEndpoint struct {
	ID           string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode  string    `gorm:"uniqueIndex;not null" json:"product_code"`
	CallbackKind string    `gorm:"index;not null" json:"callback_kind"`
	BaseURL      string    `gorm:"not null" json:"base_url"`
	Secret       string    `json:"secret"`
	Status       string    `gorm:"index;not null" json:"status"`
	Metadata     string    `gorm:"type:text" json:"metadata"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RuntimeProviderBinding struct {
	ID            string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode   string    `gorm:"index:idx_runtime_provider_binding_lookup;not null" json:"product_code"`
	TaskType      string    `gorm:"index:idx_runtime_provider_binding_lookup;not null" json:"task_type"`
	ProviderCode  string    `gorm:"index;not null" json:"provider_code"`
	Model         string    `json:"model"`
	CredentialRef string    `json:"credential_ref"`
	Priority      int       `gorm:"default:100" json:"priority"`
	Enabled       bool      `gorm:"default:true" json:"enabled"`
	Metadata      string    `gorm:"type:text" json:"metadata"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type StorageBinding struct {
	ID           string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode  string    `gorm:"index:idx_storage_binding_lookup;not null" json:"product_code"`
	Category     string    `gorm:"index:idx_storage_binding_lookup;not null" json:"category"`
	ProviderCode string    `gorm:"index;not null" json:"provider_code"`
	LocalBaseDir string    `json:"local_base_dir"`
	Priority     int       `gorm:"default:100" json:"priority"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	Metadata     string    `gorm:"type:text" json:"metadata"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type RuntimeJob struct {
	ID              string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode     string     `gorm:"index;uniqueIndex:idx_runtime_jobs_idempotency_scope;not null" json:"product_code"`
	TaskType        string     `gorm:"index;uniqueIndex:idx_runtime_jobs_idempotency_scope;not null" json:"task_type"`
	ProviderCode    string     `gorm:"index" json:"provider_code"`
	ProviderMode    string     `gorm:"index;not null" json:"provider_mode"`
	ProviderJobID   string     `gorm:"index" json:"provider_job_id"`
	OrganizationID  string     `gorm:"index;uniqueIndex:idx_runtime_jobs_idempotency_scope;not null" json:"organization_id"`
	UserID          string     `gorm:"index" json:"user_id"`
	SourceType      string     `gorm:"index;uniqueIndex:idx_runtime_jobs_idempotency_scope;not null" json:"source_type"`
	SourceID        string     `gorm:"index;uniqueIndex:idx_runtime_jobs_idempotency_scope;not null" json:"source_id"`
	IdempotencyKey  *string    `gorm:"uniqueIndex:idx_runtime_jobs_idempotency_scope" json:"idempotency_key,omitempty"`
	ChargeSessionID string     `gorm:"index" json:"charge_session_id"`
	Status          string     `gorm:"index;not null" json:"status"`
	Stage           string     `gorm:"index" json:"stage"`
	StageMessage    string     `json:"stage_message"`
	ErrorClass      string     `gorm:"index" json:"error_class"`
	ErrorCode       string     `gorm:"index" json:"error_code"`
	ErrorMessage    string     `json:"error_message"`
	InputManifest   string     `gorm:"type:text" json:"input_manifest"`
	OutputManifest  string     `gorm:"type:text" json:"output_manifest"`
	RouteSnapshot   string     `gorm:"type:text" json:"route_snapshot"`
	Metadata        string     `gorm:"type:text" json:"metadata"`
	Priority        int        `gorm:"default:0" json:"priority"`
	AttemptCount    int        `gorm:"default:0" json:"attempt_count"`
	MaxAttempts     int        `gorm:"default:3" json:"max_attempts"`
	TimeoutAt       *time.Time `gorm:"index" json:"timeout_at"`
	NextRetryAt     *time.Time `gorm:"index" json:"next_retry_at"`
	CompletedAt     *time.Time `gorm:"index" json:"completed_at"`
	CanceledAt      *time.Time `gorm:"index" json:"canceled_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type RuntimeAttempt struct {
	ID               string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	RuntimeJobID     string     `gorm:"index;not null" json:"runtime_job_id"`
	AttemptNo        int        `gorm:"not null" json:"attempt_no"`
	Status           string     `gorm:"index;not null" json:"status"`
	ErrorClass       string     `gorm:"index" json:"error_class"`
	ErrorCode        string     `gorm:"index" json:"error_code"`
	ErrorMessage     string     `json:"error_message"`
	ProviderCode     string     `gorm:"index" json:"provider_code"`
	ProviderMode     string     `gorm:"index" json:"provider_mode"`
	ProviderRequest  string     `gorm:"type:text" json:"provider_request"`
	ProviderResponse string     `gorm:"type:text" json:"provider_response"`
	StartedAt        *time.Time `gorm:"index" json:"started_at"`
	EndedAt          *time.Time `gorm:"index" json:"ended_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type ChargeSession struct {
	ID                 string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	SourceType         string     `gorm:"index;not null" json:"source_type"`
	SourceID           string     `gorm:"index;not null" json:"source_id"`
	ProductCode        string     `gorm:"index;not null" json:"product_code"`
	OrganizationID     string     `gorm:"index;not null" json:"organization_id"`
	UserID             string     `gorm:"index" json:"user_id"`
	BillingSubjectType string     `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string     `gorm:"index;not null" json:"billing_subject_id"`
	BillableItemCode   string     `gorm:"index;not null" json:"billable_item_code"`
	ResourceType       string     `gorm:"index;not null" json:"resource_type"`
	Status             string     `gorm:"index;not null" json:"status"`
	ReservationKey     string     `gorm:"uniqueIndex;not null" json:"reservation_key"`
	ReservationID      string     `gorm:"index" json:"reservation_id"`
	FinalizationID     string     `gorm:"index" json:"finalization_id"`
	EventID            string     `gorm:"index" json:"event_id"`
	SettlementID       string     `gorm:"index" json:"settlement_id"`
	EstimatedUnits     int64      `json:"estimated_units"`
	FinalUnits         int64      `json:"final_units"`
	RouteSnapshot      string     `gorm:"type:text" json:"route_snapshot"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	ReservedAt         *time.Time `gorm:"index" json:"reserved_at"`
	FinalizedAt        *time.Time `gorm:"index" json:"finalized_at"`
	ReleasedAt         *time.Time `gorm:"index" json:"released_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type RuntimeCallbackDelivery struct {
	ID            string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	RuntimeJobID  string     `gorm:"index;not null" json:"runtime_job_id"`
	ProductCode   string     `gorm:"index;not null" json:"product_code"`
	SourceID      string     `gorm:"index;not null" json:"source_id"`
	CallbackType  string     `gorm:"index;not null" json:"callback_type"`
	Status        string     `gorm:"index;not null" json:"status"`
	PayloadJSON   string     `gorm:"type:text;not null" json:"payload_json"`
	AttemptCount  int        `gorm:"default:0" json:"attempt_count"`
	MaxAttempts   int        `gorm:"default:8" json:"max_attempts"`
	LastError     string     `gorm:"type:text" json:"last_error"`
	LastAttemptAt *time.Time `gorm:"index" json:"last_attempt_at"`
	NextAttemptAt *time.Time `gorm:"index" json:"next_attempt_at"`
	DeliveredAt   *time.Time `gorm:"index" json:"delivered_at"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}
