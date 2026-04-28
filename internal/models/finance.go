package models

import "time"

type WalletAccount struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingSubjectType string    `gorm:"uniqueIndex:idx_wallet_account_subject_asset;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"uniqueIndex:idx_wallet_account_subject_asset;not null" json:"billing_subject_id"`
	AssetCode          string    `gorm:"uniqueIndex:idx_wallet_account_subject_asset;not null" json:"asset_code"`
	AssetType          string    `gorm:"index;not null" json:"asset_type"`
	Balance            int64     `json:"balance"`
	Status             string    `gorm:"index;not null" json:"status"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type AssetDefinition struct {
	AssetCode         string    `gorm:"primaryKey;type:varchar(64)" json:"asset_code"`
	ProductCode       string    `gorm:"index" json:"product_code"`
	AssetType         string    `gorm:"index;not null" json:"asset_type"`
	LifecycleType     string    `gorm:"index;not null" json:"lifecycle_type"`
	DefaultExpireDays int       `json:"default_expire_days"`
	ResetCycle        string    `gorm:"size:32" json:"reset_cycle"`
	Status            string    `gorm:"index;not null" json:"status"`
	Description       string    `json:"description"`
	Metadata          string    `gorm:"type:text" json:"metadata"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type WalletBucket struct {
	ID                 string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	WalletAccountID    string     `gorm:"index;not null" json:"wallet_account_id"`
	BillingSubjectType string     `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string     `gorm:"index;not null" json:"billing_subject_id"`
	AssetCode          string     `gorm:"index;not null" json:"asset_code"`
	AssetType          string     `gorm:"index;not null" json:"asset_type"`
	LifecycleType      string     `gorm:"index;not null" json:"lifecycle_type"`
	SourceType         string     `gorm:"index" json:"source_type"`
	SourceID           string     `gorm:"index" json:"source_id"`
	CycleKey           string     `gorm:"index" json:"cycle_key"`
	Balance            int64      `json:"balance"`
	ExpiresAt          *time.Time `gorm:"index" json:"expires_at,omitempty"`
	Status             string     `gorm:"index;not null" json:"status"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type AllowancePolicy struct {
	ID                 string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode        string     `gorm:"index;not null" json:"product_code"`
	BillingSubjectType string     `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string     `gorm:"index;not null" json:"billing_subject_id"`
	AssetCode          string     `gorm:"index;not null" json:"asset_code"`
	Amount             int64      `json:"amount"`
	ResetCycle         string     `gorm:"size:32" json:"reset_cycle"`
	Status             string     `gorm:"index;not null" json:"status"`
	EffectiveFrom      *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo        *time.Time `gorm:"index" json:"effective_to,omitempty"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type SettlementRecord struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	EventID            string    `gorm:"uniqueIndex;not null" json:"event_id"`
	RequestID          string    `gorm:"index" json:"request_id"`
	TraceID            string    `gorm:"index" json:"trace_id"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	ProductCode        string    `gorm:"index;not null" json:"product_code"`
	BillableItemCode   string    `gorm:"index;not null" json:"billable_item_code"`
	BillingProfileID   string    `gorm:"index" json:"billing_profile_id"`
	CommercialEntityID string    `gorm:"index" json:"commercial_entity_id"`
	MerchantAccountID  string    `gorm:"index" json:"merchant_account_id"`
	SettlementMode     string    `gorm:"index;not null" json:"settlement_mode"`
	Currency           string    `gorm:"size:16" json:"currency"`
	GrossAmount        int64     `json:"gross_amount"`
	DiscountAmount     int64     `json:"discount_amount"`
	NetAmount          int64     `json:"net_amount"`
	QuotaConsumed      int64     `json:"quota_consumed"`
	CreditsConsumed    int64     `json:"credits_consumed"`
	WalletAssetCode    string    `gorm:"size:64" json:"wallet_asset_code"`
	WalletDebited      int64     `json:"wallet_debited"`
	BillingAmount      int64     `json:"billing_amount"`
	RewardAmount       int64     `json:"reward_amount"`
	CommissionAmount   int64     `json:"commission_amount"`
	Status             string    `gorm:"index;not null" json:"status"`
	Snapshot           string    `gorm:"type:text" json:"snapshot"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type WalletLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	WalletAccountID    string    `gorm:"index;not null" json:"wallet_account_id"`
	WalletBucketID     string    `gorm:"index" json:"wallet_bucket_id"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	AssetCode          string    `gorm:"index;not null" json:"asset_code"`
	Direction          string    `gorm:"index;not null" json:"direction"`
	Amount             int64     `json:"amount"`
	Reason             string    `json:"reason"`
	ReferenceType      string    `gorm:"index" json:"reference_type"`
	ReferenceID        string    `gorm:"index" json:"reference_id"`
	Status             string    `gorm:"index;not null" json:"status"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
}

type DiscountLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode        string    `gorm:"index" json:"product_code"`
	CampaignCode       string    `gorm:"index" json:"campaign_code"`
	DiscountType       string    `gorm:"index;not null" json:"discount_type"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	Currency           string    `gorm:"size:16" json:"currency"`
	Amount             int64     `json:"amount"`
	Status             string    `gorm:"index;not null" json:"status"`
	ReferenceType      string    `gorm:"index" json:"reference_type"`
	ReferenceID        string    `gorm:"index" json:"reference_id"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type RewardLedger struct {
	ID                     string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode            string     `gorm:"index" json:"product_code"`
	CampaignCode           string     `gorm:"index" json:"campaign_code"`
	RewardType             string     `gorm:"index;not null" json:"reward_type"`
	BeneficiarySubjectType string     `gorm:"index;not null" json:"beneficiary_subject_type"`
	BeneficiarySubjectID   string     `gorm:"index;not null" json:"beneficiary_subject_id"`
	AssetCode              string     `gorm:"index" json:"asset_code"`
	AssetType              string     `gorm:"index" json:"asset_type"`
	LifecycleType          string     `gorm:"index" json:"lifecycle_type"`
	CycleKey               string     `gorm:"index" json:"cycle_key"`
	WalletBucketID         string     `gorm:"index" json:"wallet_bucket_id"`
	Amount                 int64      `json:"amount"`
	Status                 string     `gorm:"index;not null" json:"status"`
	ReferenceType          string     `gorm:"index" json:"reference_type"`
	ReferenceID            string     `gorm:"index" json:"reference_id"`
	ExpiresAt              *time.Time `gorm:"index" json:"expires_at,omitempty"`
	Metadata               string     `gorm:"type:text" json:"metadata"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type CommissionLedger struct {
	ID                     string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode            string     `gorm:"index" json:"product_code"`
	CommissionType         string     `gorm:"index;not null" json:"commission_type"`
	BeneficiarySubjectType string     `gorm:"index;not null" json:"beneficiary_subject_type"`
	BeneficiarySubjectID   string     `gorm:"index;not null" json:"beneficiary_subject_id"`
	SettlementSubjectType  string     `gorm:"index" json:"settlement_subject_type"`
	SettlementSubjectID    string     `gorm:"index" json:"settlement_subject_id"`
	Currency               string     `gorm:"size:64" json:"currency"`
	Amount                 int64      `json:"amount"`
	Status                 string     `gorm:"index;not null" json:"status"`
	ReferenceType          string     `gorm:"index" json:"reference_type"`
	ReferenceID            string     `gorm:"index" json:"reference_id"`
	RedeemedRewardID       string     `gorm:"index" json:"redeemed_reward_id"`
	RedeemedAt             *time.Time `json:"redeemed_at,omitempty"`
	Metadata               string     `gorm:"type:text" json:"metadata"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type ReferralProgram struct {
	ID                    string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode           string     `gorm:"index;not null" json:"product_code"`
	ProgramCode           string     `gorm:"uniqueIndex;not null" json:"program_code"`
	Name                  string     `gorm:"not null" json:"name"`
	Status                string     `gorm:"index;not null" json:"status"`
	TriggerType           string     `gorm:"index;not null" json:"trigger_type"`
	CommissionPolicy      string     `gorm:"index;not null" json:"commission_policy"`
	CommissionCurrency    string     `gorm:"size:64" json:"commission_currency"`
	CommissionFixedAmount int64      `json:"commission_fixed_amount"`
	CommissionRateBps     int64      `json:"commission_rate_bps"`
	SettlementDelayDays   int        `json:"settlement_delay_days"`
	AllowRepeat           bool       `json:"allow_repeat"`
	EffectiveFrom         *time.Time `gorm:"index" json:"effective_from,omitempty"`
	EffectiveTo           *time.Time `gorm:"index" json:"effective_to,omitempty"`
	Metadata              string     `gorm:"type:text" json:"metadata"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
}

type ReferralCode struct {
	ID                  string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProgramID           string    `gorm:"index;not null" json:"program_id"`
	ProductCode         string    `gorm:"index;not null" json:"product_code"`
	Code                string    `gorm:"uniqueIndex;not null" json:"code"`
	PromoterSubjectType string    `gorm:"index;not null" json:"promoter_subject_type"`
	PromoterSubjectID   string    `gorm:"index;not null" json:"promoter_subject_id"`
	Status              string    `gorm:"index;not null" json:"status"`
	Metadata            string    `gorm:"type:text" json:"metadata"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

type ReferralConversion struct {
	ID                    string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProgramID             string    `gorm:"index;not null" json:"program_id"`
	ReferralCodeID        string    `gorm:"index;not null" json:"referral_code_id"`
	ProductCode           string    `gorm:"index;not null" json:"product_code"`
	TriggerType           string    `gorm:"index;not null" json:"trigger_type"`
	PromoterSubjectType   string    `gorm:"index;not null" json:"promoter_subject_type"`
	PromoterSubjectID     string    `gorm:"index;not null" json:"promoter_subject_id"`
	ReferredSubjectType   string    `gorm:"index;not null" json:"referred_subject_type"`
	ReferredSubjectID     string    `gorm:"index;not null" json:"referred_subject_id"`
	SettlementSubjectType string    `gorm:"index" json:"settlement_subject_type"`
	SettlementSubjectID   string    `gorm:"index" json:"settlement_subject_id"`
	ReferenceType         string    `gorm:"index;not null;uniqueIndex:idx_referral_conversion_ref" json:"reference_type"`
	ReferenceID           string    `gorm:"index;not null;uniqueIndex:idx_referral_conversion_ref" json:"reference_id"`
	CommissionCurrency    string    `gorm:"size:64" json:"commission_currency"`
	CommissionAmount      int64     `json:"commission_amount"`
	CommissionLedgerID    string    `gorm:"index" json:"commission_ledger_id"`
	Status                string    `gorm:"index;not null" json:"status"`
	Metadata              string    `gorm:"type:text" json:"metadata"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}
