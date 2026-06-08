package models

import "time"

type Product struct {
	ID        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Code      string    `gorm:"uniqueIndex;not null" json:"code"`
	Name      string    `gorm:"not null" json:"name"`
	Status    string    `gorm:"index;not null" json:"status"`
	OwnerTeam string    `json:"owner_team"`
	Metadata  string    `gorm:"type:text" json:"metadata"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type SKU struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductID   string    `gorm:"index;not null" json:"product_id"`
	Code        string    `gorm:"uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"not null" json:"name"`
	SKUType     string    `gorm:"index;not null" json:"sku_type"`
	BillingMode string    `gorm:"index;not null" json:"billing_mode"`
	Currency    string    `gorm:"size:16" json:"currency"`
	ListPrice   int64     `json:"list_price"`
	Status      string    `gorm:"index;not null" json:"status"`
	Metadata    string    `gorm:"type:text" json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CommercialPackage struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductID   string    `gorm:"index;not null" json:"product_id"`
	Code        string    `gorm:"uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"not null" json:"name"`
	PackageType string    `gorm:"index;not null" json:"package_type"`
	Status      string    `gorm:"index;not null" json:"status"`
	Metadata    string    `gorm:"type:text" json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type BillableItem struct {
	ID              string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductID       string    `gorm:"index;not null" json:"product_id"`
	Code            string    `gorm:"uniqueIndex;not null" json:"code"`
	Name            string    `gorm:"not null" json:"name"`
	MeterUnit       string    `gorm:"index;not null" json:"meter_unit"`
	BillingScope    string    `gorm:"index;not null" json:"billing_scope"`
	SettlementMode  string    `gorm:"index;not null" json:"settlement_mode"`
	PricingBehavior string    `gorm:"index;not null" json:"pricing_behavior"`
	Status          string    `gorm:"index;not null" json:"status"`
	Metadata        string    `gorm:"type:text" json:"metadata"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type RateCard struct {
	ID            string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductID     string     `gorm:"index;not null" json:"product_id"`
	Code          string     `gorm:"uniqueIndex;not null" json:"code"`
	TargetType    string     `gorm:"index;not null" json:"target_type"`
	TargetID      string     `gorm:"index;not null" json:"target_id"`
	PriceModel    string     `gorm:"index;not null" json:"price_model"`
	Currency      string     `gorm:"size:16" json:"currency"`
	PriceConfig   string     `gorm:"type:text" json:"price_config"`
	EffectiveFrom *time.Time `json:"effective_from"`
	EffectiveTo   *time.Time `json:"effective_to"`
	Version       int        `gorm:"default:1" json:"version"`
	Status        string     `gorm:"index;not null" json:"status"`
	Metadata      string     `gorm:"type:text" json:"metadata"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type QuotaGrantPolicy struct {
	ID               string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode      string    `gorm:"uniqueIndex:idx_quota_grant_policy_key;not null" json:"product_code"`
	PackageCode      string    `gorm:"uniqueIndex:idx_quota_grant_policy_key;not null" json:"package_code"`
	BillableItemCode string    `gorm:"uniqueIndex:idx_quota_grant_policy_key;not null" json:"billable_item_code"`
	GrantMode        string    `gorm:"type:varchar(32);not null" json:"grant_mode"`
	Units            int64     `gorm:"not null;default:0" json:"units"`
	ResetCycle       string    `gorm:"type:varchar(32)" json:"reset_cycle"`
	Status           string    `gorm:"index;not null" json:"status"`
	Metadata         string    `gorm:"type:text" json:"metadata"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type PackageCapabilityPolicy struct {
	ID             string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode    string    `gorm:"uniqueIndex:idx_package_capability_policy_key;not null" json:"product_code"`
	PackageCode    string    `gorm:"uniqueIndex:idx_package_capability_policy_key;not null" json:"package_code"`
	CapabilityCode string    `gorm:"uniqueIndex:idx_package_capability_policy_key;not null" json:"capability_code"`
	GrantValue     string    `gorm:"type:varchar(128);not null" json:"grant_value"`
	Status         string    `gorm:"index;not null" json:"status"`
	Metadata       string    `gorm:"type:text" json:"metadata"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type CapabilityGrant struct {
	ID                 string     `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode        string     `gorm:"index;not null" json:"product_code"`
	BillingSubjectType string     `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string     `gorm:"index;not null" json:"billing_subject_id"`
	CapabilityCode     string     `gorm:"index;not null" json:"capability_code"`
	GrantValue         string     `gorm:"type:varchar(128);not null" json:"grant_value"`
	SourceType         string     `gorm:"type:varchar(64);index" json:"source_type"`
	SourceID           string     `gorm:"type:varchar(191);index" json:"source_id"`
	Status             string     `gorm:"index;not null" json:"status"`
	ExpiresAt          *time.Time `json:"expires_at,omitempty"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type CommercialEntity struct {
	ID          string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Code        string    `gorm:"uniqueIndex;not null" json:"code"`
	Name        string    `gorm:"not null" json:"name"`
	EntityType  string    `gorm:"index;not null" json:"entity_type"`
	CountryCode string    `gorm:"size:16" json:"country_code"`
	Currency    string    `gorm:"size:16" json:"currency"`
	TaxProfile  string    `gorm:"type:text" json:"tax_profile"`
	Status      string    `gorm:"index;not null" json:"status"`
	Metadata    string    `gorm:"type:text" json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MerchantAccount struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	CommercialEntityID string    `gorm:"index;not null" json:"commercial_entity_id"`
	Channel            string    `gorm:"index;not null" json:"channel"`
	AccountCode        string    `gorm:"uniqueIndex;not null" json:"account_code"`
	AccountName        string    `json:"account_name"`
	CountryCode        string    `gorm:"size:16" json:"country_code"`
	Currency           string    `gorm:"size:16" json:"currency"`
	Capabilities       string    `gorm:"type:text" json:"capabilities"`
	Status             string    `gorm:"index;not null" json:"status"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type SettlementAccount struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	CommercialEntityID string    `gorm:"index;not null" json:"commercial_entity_id"`
	AccountType        string    `gorm:"index;not null" json:"account_type"`
	Currency           string    `gorm:"size:16" json:"currency"`
	BankRegion         string    `json:"bank_region"`
	MaskedAccountNo    string    `json:"masked_account_no"`
	Status             string    `gorm:"index;not null" json:"status"`
	Metadata           string    `gorm:"type:text" json:"metadata"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type BillingProfile struct {
	ID                         string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	Code                       string    `gorm:"uniqueIndex;not null" json:"code"`
	ProductID                  string    `gorm:"index;not null" json:"product_id"`
	CommercialEntityID         string    `gorm:"index;not null" json:"commercial_entity_id"`
	DefaultMerchantAccountID   string    `gorm:"index" json:"default_merchant_account_id"`
	DefaultSettlementAccountID string    `gorm:"index" json:"default_settlement_account_id"`
	RegionScope                string    `json:"region_scope"`
	Currency                   string    `gorm:"size:16" json:"currency"`
	PricingStrategy            string    `json:"pricing_strategy"`
	TaxStrategy                string    `json:"tax_strategy"`
	Status                     string    `gorm:"index;not null" json:"status"`
	Metadata                   string    `gorm:"type:text" json:"metadata"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type RoutingPolicy struct {
	ID                        string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingProfileID          string    `gorm:"index;not null" json:"billing_profile_id"`
	Priority                  int       `gorm:"index;not null" json:"priority"`
	MatchType                 string    `gorm:"index;not null" json:"match_type"`
	MatchConfig               string    `gorm:"type:text" json:"match_config"`
	TargetMerchantAccountID   string    `gorm:"index" json:"target_merchant_account_id"`
	TargetSettlementAccountID string    `gorm:"index" json:"target_settlement_account_id"`
	Status                    string    `gorm:"index;not null" json:"status"`
	Metadata                  string    `gorm:"type:text" json:"metadata"`
	CreatedAt                 time.Time `json:"created_at"`
	UpdatedAt                 time.Time `json:"updated_at"`
}

type OrgBillingProfile struct {
	ID               string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	OrganizationID   string    `gorm:"uniqueIndex:idx_org_billing_profile;not null" json:"organization_id"`
	BillingProfileID string    `gorm:"index;not null" json:"billing_profile_id"`
	IsDefault        bool      `gorm:"default:true" json:"is_default"`
	Status           string    `gorm:"index;not null" json:"status"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type MeterEvent struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	EventID            string    `gorm:"uniqueIndex;not null" json:"event_id"`
	RequestID          string    `gorm:"index" json:"request_id"`
	TraceID            string    `gorm:"index" json:"trace_id"`
	SourceType         string    `gorm:"index" json:"source_type"`
	SourceID           string    `json:"source_id"`
	SourceAction       string    `gorm:"index" json:"source_action"`
	ProductCode        string    `gorm:"index;not null" json:"product_code"`
	OrgID              string    `gorm:"index;not null" json:"org_id"`
	UserID             string    `gorm:"index" json:"user_id"`
	BillableItemCode   string    `gorm:"index;not null" json:"billable_item_code"`
	ChargeGroupID      string    `gorm:"index" json:"charge_group_id"`
	ParentEventID      string    `gorm:"index" json:"parent_event_id"`
	EventRole          string    `gorm:"index" json:"event_role"`
	BillingSubjectType string    `gorm:"index" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index" json:"billing_subject_id"`
	UsageUnits         int64     `json:"usage_units"`
	Unit               string    `json:"unit"`
	Billable           bool      `gorm:"index" json:"billable"`
	BillingProfileKey  string    `gorm:"index" json:"billing_profile_key"`
	CurrencyContext    string    `json:"currency_context"`
	Dimensions         string    `gorm:"type:text" json:"dimensions"`
	OccurredAt         time.Time `gorm:"index" json:"occurred_at"`
	ReceivedAt         time.Time `gorm:"index" json:"received_at"`
	Status             string    `gorm:"index;not null" json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type UsageRecord struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	EventID            string    `gorm:"uniqueIndex;not null" json:"event_id"`
	RequestID          string    `gorm:"index" json:"request_id"`
	TraceID            string    `gorm:"index" json:"trace_id"`
	ProductCode        string    `gorm:"index;not null" json:"product_code"`
	OrgID              string    `gorm:"index;not null" json:"org_id"`
	UserID             string    `gorm:"index" json:"user_id"`
	BillableItemCode   string    `gorm:"index;not null" json:"billable_item_code"`
	ChargeGroupID      string    `gorm:"index" json:"charge_group_id"`
	EventRole          string    `gorm:"index" json:"event_role"`
	BillingSubjectType string    `gorm:"index" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index" json:"billing_subject_id"`
	UsageUnits         int64     `json:"usage_units"`
	Billable           bool      `gorm:"index" json:"billable"`
	BillingProfileID   string    `gorm:"index" json:"billing_profile_id"`
	CommercialEntityID string    `gorm:"index" json:"commercial_entity_id"`
	MerchantAccountID  string    `gorm:"index" json:"merchant_account_id"`
	Dimensions         string    `gorm:"type:text" json:"dimensions"`
	OccurredAt         time.Time `gorm:"index" json:"occurred_at"`
	RecordedAt         time.Time `gorm:"index" json:"recorded_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type UsageAgg struct {
	ID               string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	ProductCode      string    `gorm:"uniqueIndex:idx_usage_agg_rollup;not null" json:"product_code"`
	OrgID            string    `gorm:"uniqueIndex:idx_usage_agg_rollup;not null" json:"org_id"`
	BillableItemCode string    `gorm:"uniqueIndex:idx_usage_agg_rollup;not null" json:"billable_item_code"`
	TimeGranularity  string    `gorm:"uniqueIndex:idx_usage_agg_rollup;not null" json:"time_granularity"`
	StatTime         time.Time `gorm:"uniqueIndex:idx_usage_agg_rollup;not null" json:"stat_time"`
	Dimensions       string    `gorm:"uniqueIndex:idx_usage_agg_rollup;type:text" json:"dimensions"`
	UsageUnits       int64     `json:"usage_units"`
	EventCount       int64     `json:"event_count"`
	BillableUnits    int64     `json:"billable_units"`
	UpdatedAt        time.Time `json:"updated_at"`
	CreatedAt        time.Time `json:"created_at"`
}

type QuotaLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	BillableItemCode   string    `gorm:"index;not null" json:"billable_item_code"`
	Direction          string    `gorm:"index;not null" json:"direction"`
	Units              int64     `json:"units"`
	Reason             string    `json:"reason"`
	ReferenceID        string    `gorm:"index" json:"reference_id"`
	CreatedAt          time.Time `json:"created_at"`
}

type QuotaBalance struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingSubjectType string    `gorm:"uniqueIndex:idx_quota_balance_subject_item;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"uniqueIndex:idx_quota_balance_subject_item;not null" json:"billing_subject_id"`
	BillableItemCode   string    `gorm:"uniqueIndex:idx_quota_balance_subject_item;not null" json:"billable_item_code"`
	AvailableUnits     int64     `json:"available_units"`
	LedgerSyncedAt     time.Time `json:"ledger_synced_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type CreditsLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	Direction          string    `gorm:"index;not null" json:"direction"`
	Amount             int64     `json:"amount"`
	Reason             string    `json:"reason"`
	ReferenceID        string    `gorm:"index" json:"reference_id"`
	CreatedAt          time.Time `json:"created_at"`
}

type BillingLedger struct {
	ID                 string    `gorm:"primaryKey;type:varchar(64)" json:"id"`
	BillingSubjectType string    `gorm:"index;not null" json:"billing_subject_type"`
	BillingSubjectID   string    `gorm:"index;not null" json:"billing_subject_id"`
	ProductCode        string    `gorm:"index;not null" json:"product_code"`
	BillableItemCode   string    `gorm:"index;not null" json:"billable_item_code"`
	Currency           string    `gorm:"size:16" json:"currency"`
	Amount             int64     `json:"amount"`
	Direction          string    `gorm:"index;not null" json:"direction"`
	Status             string    `gorm:"index;not null" json:"status"`
	ReferenceID        string    `gorm:"index" json:"reference_id"`
	OccurredAt         time.Time `gorm:"index" json:"occurred_at"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}
