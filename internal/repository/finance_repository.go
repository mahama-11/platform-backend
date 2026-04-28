package repository

import (
	"platform-service/internal/models"
	"platform-service/pkg/platformconst"
	"time"

	"gorm.io/gorm"
)

type FinanceRepository struct {
	db *gorm.DB
}

func NewFinanceRepository(db *gorm.DB) *FinanceRepository {
	return &FinanceRepository{db: db}
}

func (r *FinanceRepository) DB() *gorm.DB { return r.db }

func (r *FinanceRepository) CreateWalletAccount(item *models.WalletAccount) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) CreateAssetDefinition(item *models.AssetDefinition) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveAssetDefinition(item *models.AssetDefinition) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindAssetDefinition(assetCode string) (*models.AssetDefinition, error) {
	var item models.AssetDefinition
	if err := r.db.Where("asset_code = ?", assetCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) DeleteAssetDefinition(assetCode string) error {
	return r.db.Delete(&models.AssetDefinition{}, "asset_code = ?", assetCode).Error
}

func (r *FinanceRepository) ListAssetDefinitions(productCode, lifecycleType, status string) ([]models.AssetDefinition, error) {
	var items []models.AssetDefinition
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if lifecycleType != "" {
		q = q.Where("lifecycle_type = ?", lifecycleType)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateAllowancePolicy(item *models.AllowancePolicy) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveAllowancePolicy(item *models.AllowancePolicy) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindAllowancePolicy(productCode, subjectType, subjectID, assetCode string) (*models.AllowancePolicy, error) {
	var item models.AllowancePolicy
	if err := r.db.Where("product_code = ? AND billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", productCode, subjectType, subjectID, assetCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindAllowancePolicyByID(id string) (*models.AllowancePolicy, error) {
	var item models.AllowancePolicy
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListAllowancePolicies(productCode, assetCode, status string) ([]models.AllowancePolicy, error) {
	var items []models.AllowancePolicy
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if assetCode != "" {
		q = q.Where("asset_code = ?", assetCode)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) DeleteAllowancePolicy(id string) error {
	return r.db.Delete(&models.AllowancePolicy{}, "id = ?", id).Error
}

func (r *FinanceRepository) CreateSettlementRecord(item *models.SettlementRecord) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindSettlementRecordByEventID(eventID string) (*models.SettlementRecord, error) {
	var item models.SettlementRecord
	if err := r.db.Where("event_id = ?", eventID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListSettlementRecords(subjectType, subjectID, productCode string) ([]models.SettlementRecord, error) {
	var items []models.SettlementRecord
	q := r.db.Order("created_at desc")
	if subjectType != "" {
		q = q.Where("billing_subject_type = ?", subjectType)
	}
	if subjectID != "" {
		q = q.Where("billing_subject_id = ?", subjectID)
	}
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) SaveWalletAccount(item *models.WalletAccount) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindWalletAccountByID(id string) (*models.WalletAccount, error) {
	var item models.WalletAccount
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindWalletAccount(subjectType, subjectID, assetCode string) (*models.WalletAccount, error) {
	var item models.WalletAccount
	if err := r.db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListWalletAccounts(subjectType, subjectID string) ([]models.WalletAccount, error) {
	var items []models.WalletAccount
	q := r.db.Order("created_at desc")
	if subjectType != "" {
		q = q.Where("billing_subject_type = ?", subjectType)
	}
	if subjectID != "" {
		q = q.Where("billing_subject_id = ?", subjectID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateWalletLedger(item *models.WalletLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) FindWalletLedgerByReference(subjectType, subjectID, assetCode, direction, referenceType, referenceID string) (*models.WalletLedger, error) {
	var item models.WalletLedger
	if err := r.db.Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ? AND direction = ? AND reference_type = ? AND reference_id = ?", subjectType, subjectID, assetCode, direction, referenceType, referenceID).
		Order("created_at desc").
		First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) CreateWalletBucket(item *models.WalletBucket) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveWalletBucket(item *models.WalletBucket) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindWalletBucketByID(id string) (*models.WalletBucket, error) {
	var item models.WalletBucket
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindWalletBucketByCycle(walletAccountID, cycleKey string) (*models.WalletBucket, error) {
	var item models.WalletBucket
	if err := r.db.Where("wallet_account_id = ? AND cycle_key = ?", walletAccountID, cycleKey).Order("created_at desc").First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListWalletBuckets(walletAccountID, status string) ([]models.WalletBucket, error) {
	var items []models.WalletBucket
	q := r.db.Order("expires_at asc, created_at asc")
	if walletAccountID != "" {
		q = q.Where("wallet_account_id = ?", walletAccountID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListSpendableWalletBuckets(walletAccountID string, now time.Time) ([]models.WalletBucket, error) {
	var items []models.WalletBucket
	err := r.db.
		Where("wallet_account_id = ? AND status = ? AND balance > 0", walletAccountID, platformconst.StatusActive).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("CASE WHEN expires_at IS NULL THEN 1 ELSE 0 END, expires_at asc, created_at asc").
		Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListExpirableWalletBuckets(now time.Time, assetCode string) ([]models.WalletBucket, error) {
	var items []models.WalletBucket
	q := r.db.Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ? AND balance > 0", platformconst.StatusActive, now)
	if assetCode != "" {
		q = q.Where("asset_code = ?", assetCode)
	}
	err := q.Order("expires_at asc, created_at asc").Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateDiscountLedger(item *models.DiscountLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) ListDiscountLedgers(productCode, subjectType, subjectID string) ([]models.DiscountLedger, error) {
	var items []models.DiscountLedger
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if subjectType != "" {
		q = q.Where("billing_subject_type = ?", subjectType)
	}
	if subjectID != "" {
		q = q.Where("billing_subject_id = ?", subjectID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) ListWalletLedger(walletAccountID string) ([]models.WalletLedger, error) {
	var items []models.WalletLedger
	q := r.db.Order("created_at desc")
	if walletAccountID != "" {
		q = q.Where("wallet_account_id = ?", walletAccountID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateRewardLedger(item *models.RewardLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveRewardLedger(item *models.RewardLedger) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindRewardLedgerByID(id string) (*models.RewardLedger, error) {
	var item models.RewardLedger
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListRewardLedgers(productCode, beneficiaryType, beneficiaryID string) ([]models.RewardLedger, error) {
	var items []models.RewardLedger
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if beneficiaryType != "" {
		q = q.Where("beneficiary_subject_type = ?", beneficiaryType)
	}
	if beneficiaryID != "" {
		q = q.Where("beneficiary_subject_id = ?", beneficiaryID)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateCommissionLedger(item *models.CommissionLedger) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveCommissionLedger(item *models.CommissionLedger) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindCommissionLedgerByID(id string) (*models.CommissionLedger, error) {
	var item models.CommissionLedger
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListCommissionLedgers(productCode, beneficiaryType, beneficiaryID, status string) ([]models.CommissionLedger, error) {
	var items []models.CommissionLedger
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if beneficiaryType != "" {
		q = q.Where("beneficiary_subject_type = ?", beneficiaryType)
	}
	if beneficiaryID != "" {
		q = q.Where("beneficiary_subject_id = ?", beneficiaryID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateReferralProgram(item *models.ReferralProgram) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveReferralProgram(item *models.ReferralProgram) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindReferralProgramByID(id string) (*models.ReferralProgram, error) {
	var item models.ReferralProgram
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindReferralProgramByCode(programCode string) (*models.ReferralProgram, error) {
	var item models.ReferralProgram
	if err := r.db.Where("program_code = ?", programCode).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListReferralPrograms(productCode, status string) ([]models.ReferralProgram, error) {
	var items []models.ReferralProgram
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateReferralCode(item *models.ReferralCode) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveReferralCode(item *models.ReferralCode) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindReferralCodeByCode(code string) (*models.ReferralCode, error) {
	var item models.ReferralCode
	if err := r.db.Where("code = ?", code).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListReferralCodes(programID, promoterType, promoterID, status string) ([]models.ReferralCode, error) {
	var items []models.ReferralCode
	q := r.db.Order("created_at desc")
	if programID != "" {
		q = q.Where("program_id = ?", programID)
	}
	if promoterType != "" {
		q = q.Where("promoter_subject_type = ?", promoterType)
	}
	if promoterID != "" {
		q = q.Where("promoter_subject_id = ?", promoterID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}

func (r *FinanceRepository) CreateReferralConversion(item *models.ReferralConversion) error {
	return r.db.Create(item).Error
}

func (r *FinanceRepository) SaveReferralConversion(item *models.ReferralConversion) error {
	return r.db.Save(item).Error
}

func (r *FinanceRepository) FindReferralConversionByID(id string) (*models.ReferralConversion, error) {
	var item models.ReferralConversion
	if err := r.db.Where("id = ?", id).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindReferralConversionByReference(referenceType, referenceID string) (*models.ReferralConversion, error) {
	var item models.ReferralConversion
	if err := r.db.Where("reference_type = ? AND reference_id = ?", referenceType, referenceID).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) FindReferralConversionByTriggerAndSubject(productCode, triggerType, referredType, referredID string) (*models.ReferralConversion, error) {
	var item models.ReferralConversion
	if err := r.db.Where(
		"product_code = ? AND trigger_type = ? AND referred_subject_type = ? AND referred_subject_id = ?",
		productCode, triggerType, referredType, referredID,
	).First(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *FinanceRepository) ListReferralConversions(productCode, promoterType, promoterID, status string) ([]models.ReferralConversion, error) {
	var items []models.ReferralConversion
	q := r.db.Order("created_at desc")
	if productCode != "" {
		q = q.Where("product_code = ?", productCode)
	}
	if promoterType != "" {
		q = q.Where("promoter_subject_type = ?", promoterType)
	}
	if promoterID != "" {
		q = q.Where("promoter_subject_id = ?", promoterID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Find(&items).Error
	return items, err
}
