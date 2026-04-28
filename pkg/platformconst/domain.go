package platformconst

const (
	PermissionPlatformAdmin = "platform.admin"
)

const (
	SubjectTypeOrganization = "organization"
)

const (
	MeterUnitRequest    = "request"
	MeterEventRoleEntry = "entry"
)

const (
	ResourceTypeQuota   = "quota"
	ResourceTypeCredits = "credits"
)

const (
	SettlementModeQuota               = "quota"
	SettlementModeCredits             = "credits"
	SettlementModeIncludedThenOverage = "included_then_overage"
	SettlementModeUsageBilling        = "usage_billing"
)

const (
	ReservationStatusReserved  = "reserved"
	ReservationStatusCommitted = "committed"
	ReservationStatusFinalized = "finalized"
	ReservationStatusReleased  = "released"
)

const (
	SettlementStatusSettled  = "settled"
	SettlementStatusReversed = "reversed"
)

const (
	LedgerDirectionGrant   = "grant"
	LedgerDirectionConsume = "consume"
	LedgerDirectionRefund  = "refund"
	LedgerDirectionCredit  = "credit"
	LedgerDirectionDebit   = "debit"
)

const (
	BillingLedgerStatusBooked   = "booked"
	DiscountLedgerStatusApplied = "applied"
	CommissionStatusEarned      = "earned"
	CommissionStatusRedeemed    = "redeemed"
	RewardStatusIssued          = "issued"
	WalletLedgerStatusPosted    = "posted"
	WalletBucketStatusExpired   = "expired"
)

const (
	ReferralTriggerSignup   = "signup"
	ReferralReferenceSignup = "signup"
	ReferralReferenceOrder  = "order"
)

const (
	ChannelRateTypeFixedRate   = "fixed_rate"
	ChannelTriggerChargeRecord = "charge_recorded"
	ChannelMatchPartner        = "partner"
	ChannelMatchBinding        = "binding"
)

const (
	WalletAssetTypeCredit            = "wallet_credit"
	WalletAssetTypeRewardCredit      = "reward_credit"
	WalletAssetTypeSubscriptionAllow = "subscription_allowance"
)

const (
	WalletLifecyclePermanent = "permanent"
	WalletLifecycleExpiring  = "expiring"
	WalletLifecycleCycleReset = "cycle_reset"
)
