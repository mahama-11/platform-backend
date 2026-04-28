package platformconst

const (
	StatusActive               = "active"
	StatusInactive             = "inactive"
	StatusDisabled             = "disabled"
	StatusCreated              = "created"
	StatusGenerated            = "generated"
	StatusQueued               = "queued"
	StatusPending              = "pending"
	StatusProcessing           = "processing"
	StatusAccepted             = "accepted"
	StatusCompleted            = "completed"
	StatusClosed               = "closed"
	StatusFailed               = "failed"
	StatusCanceled             = "canceled"
	StatusConfirmed            = "confirmed"
	StatusDraft                = "draft"
	StatusPublished            = "published"
	StatusArchived             = "archived"
	StatusRunning              = "running"
	StatusSuccess              = "success"
	StatusError                = "error"
	StatusRedeemed             = "redeemed"
	StatusApplied              = "applied"
	StatusSettlementInProgress = "settlement_in_progress"
)

const (
	InternalAuthModeSharedSecret = "shared-secret"
	InternalAuthModeHMAC         = "hmac"
	InternalServiceLegacySecret  = "legacy-shared-secret"
)
