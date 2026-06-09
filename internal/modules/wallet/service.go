package wallet

import (
	"context"
	"errors"
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrInsufficientWalletBalance = errors.New("insufficient wallet balance")

type DebitBreakdown struct {
	AssetCode string `json:"asset_code"`
	Amount    int64  `json:"amount"`
}

type Service struct {
	repo *repository.FinanceRepository
}

type CreateAssetDefinitionInput struct {
	AssetCode         string `json:"asset_code" binding:"required"`
	ProductCode       string `json:"product_code"`
	AssetType         string `json:"asset_type" binding:"required"`
	LifecycleType     string `json:"lifecycle_type" binding:"required"`
	DefaultExpireDays int    `json:"default_expire_days"`
	ResetCycle        string `json:"reset_cycle"`
	Status            string `json:"status"`
	Description       string `json:"description"`
	Metadata          string `json:"metadata"`
}

type UpdateAssetDefinitionInput struct {
	ProductCode       string `json:"product_code"`
	AssetType         string `json:"asset_type"`
	LifecycleType     string `json:"lifecycle_type"`
	DefaultExpireDays *int   `json:"default_expire_days"`
	ResetCycle        string `json:"reset_cycle"`
	Status            string `json:"status"`
	Description       string `json:"description"`
	Metadata          string `json:"metadata"`
}

type CreateWalletAccountInput struct {
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	AssetCode          string `json:"asset_code" binding:"required"`
	AssetType          string `json:"asset_type" binding:"required"`
	Status             string `json:"status"`
	Metadata           string `json:"metadata"`
}

type PostWalletLedgerInput struct {
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	AssetCode          string `json:"asset_code" binding:"required"`
	AssetType          string `json:"asset_type"`
	Direction          string `json:"direction" binding:"required"`
	Amount             int64  `json:"amount" binding:"required"`
	Reason             string `json:"reason"`
	ReferenceType      string `json:"reference_type"`
	ReferenceID        string `json:"reference_id"`
	Status             string `json:"status"`
	Metadata           string `json:"metadata"`
	ExpiresAt          string `json:"expires_at,omitempty"`
	CycleKey           string `json:"cycle_key,omitempty"`
}

type GrantCycleAllowanceInput struct {
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	AssetCode          string `json:"asset_code" binding:"required"`
	CycleKey           string `json:"cycle_key" binding:"required"`
	Amount             int64  `json:"amount" binding:"required"`
	Metadata           string `json:"metadata"`
}

type CreateAllowancePolicyInput struct {
	ProductCode        string `json:"product_code" binding:"required"`
	BillingSubjectType string `json:"billing_subject_type" binding:"required"`
	BillingSubjectID   string `json:"billing_subject_id" binding:"required"`
	AssetCode          string `json:"asset_code" binding:"required"`
	Amount             int64  `json:"amount" binding:"required"`
	ResetCycle         string `json:"reset_cycle,omitempty"`
	Status             string `json:"status,omitempty"`
	EffectiveFrom      string `json:"effective_from,omitempty"`
	EffectiveTo        string `json:"effective_to,omitempty"`
	Metadata           string `json:"metadata,omitempty"`
}

type UpdateAllowancePolicyInput struct {
	ProductCode        string `json:"product_code"`
	BillingSubjectType string `json:"billing_subject_type"`
	BillingSubjectID   string `json:"billing_subject_id"`
	AssetCode          string `json:"asset_code"`
	Amount             *int64 `json:"amount"`
	ResetCycle         string `json:"reset_cycle,omitempty"`
	Status             string `json:"status,omitempty"`
	EffectiveFrom      string `json:"effective_from,omitempty"`
	EffectiveTo        string `json:"effective_to,omitempty"`
	Metadata           string `json:"metadata,omitempty"`
}

type WalletAssetSummary struct {
	AssetCode        string     `json:"asset_code"`
	AssetType        string     `json:"asset_type"`
	LifecycleType    string     `json:"lifecycle_type"`
	AccountBalance   int64      `json:"account_balance"`
	AvailableBalance int64      `json:"available_balance"`
	ExpiringBalance  int64      `json:"expiring_balance"`
	NextExpiresAt    *time.Time `json:"next_expires_at,omitempty"`
}

type WalletSummary struct {
	BillingSubjectType string               `json:"billing_subject_type"`
	BillingSubjectID   string               `json:"billing_subject_id"`
	ProductCode        string               `json:"product_code"`
	TotalBalance       int64                `json:"total_balance"`
	PermanentBalance   int64                `json:"permanent_balance"`
	RewardBalance      int64                `json:"reward_balance"`
	AllowanceBalance   int64                `json:"allowance_balance"`
	Assets             []WalletAssetSummary `json:"assets"`
}

type LifecycleRunResult struct {
	ExpiredBucketCount int      `json:"expired_bucket_count"`
	GrantedPolicyCount int      `json:"granted_policy_count"`
	GrantedBucketIDs   []string `json:"granted_bucket_ids,omitempty"`
}

func NewService(repo *repository.FinanceRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateAssetDefinition(input CreateAssetDefinitionInput) (*models.AssetDefinition, error) {
	item := &models.AssetDefinition{
		AssetCode:         input.AssetCode,
		ProductCode:       input.ProductCode,
		AssetType:         input.AssetType,
		LifecycleType:     input.LifecycleType,
		DefaultExpireDays: input.DefaultExpireDays,
		ResetCycle:        input.ResetCycle,
		Status:            defaultString(input.Status, platformconst.StatusActive),
		Description:       input.Description,
		Metadata:          input.Metadata,
	}
	if existing, err := s.repo.FindAssetDefinition(input.AssetCode); err == nil {
		existing.ProductCode = item.ProductCode
		existing.AssetType = item.AssetType
		existing.LifecycleType = item.LifecycleType
		existing.DefaultExpireDays = item.DefaultExpireDays
		existing.ResetCycle = item.ResetCycle
		existing.Status = item.Status
		existing.Description = item.Description
		existing.Metadata = item.Metadata
		existing.UpdatedAt = time.Now()
		return existing, s.repo.SaveAssetDefinition(existing)
	}
	if err := s.repo.CreateAssetDefinition(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListAssetDefinitions(productCode, lifecycleType, status string) ([]models.AssetDefinition, error) {
	return s.repo.ListAssetDefinitions(productCode, lifecycleType, status)
}

func (s *Service) GetAssetDefinition(assetCode string) (*models.AssetDefinition, error) {
	return s.repo.FindAssetDefinition(assetCode)
}

func (s *Service) UpdateAssetDefinition(assetCode string, input UpdateAssetDefinitionInput) (*models.AssetDefinition, error) {
	item, err := s.repo.FindAssetDefinition(assetCode)
	if err != nil {
		return nil, err
	}
	if input.ProductCode != "" {
		item.ProductCode = input.ProductCode
	}
	if input.AssetType != "" {
		item.AssetType = input.AssetType
	}
	if input.LifecycleType != "" {
		item.LifecycleType = input.LifecycleType
	}
	if input.DefaultExpireDays != nil {
		item.DefaultExpireDays = *input.DefaultExpireDays
	}
	if input.ResetCycle != "" {
		item.ResetCycle = input.ResetCycle
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Description != "" {
		item.Description = input.Description
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	return item, s.repo.SaveAssetDefinition(item)
}

func (s *Service) DeleteAssetDefinition(assetCode string) (*models.AssetDefinition, error) {
	item, err := s.repo.FindAssetDefinition(assetCode)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteAssetDefinition(assetCode); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateAllowancePolicy(input CreateAllowancePolicyInput) (*models.AllowancePolicy, error) {
	item := &models.AllowancePolicy{
		ID:                 utils.GenerateID(),
		ProductCode:        input.ProductCode,
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		AssetCode:          input.AssetCode,
		Amount:             input.Amount,
		ResetCycle:         input.ResetCycle,
		Status:             defaultString(input.Status, platformconst.StatusActive),
		Metadata:           input.Metadata,
	}
	if input.EffectiveFrom != "" {
		parsed, err := time.Parse(time.RFC3339, input.EffectiveFrom)
		if err != nil {
			return nil, err
		}
		item.EffectiveFrom = &parsed
	}
	if input.EffectiveTo != "" {
		parsed, err := time.Parse(time.RFC3339, input.EffectiveTo)
		if err != nil {
			return nil, err
		}
		item.EffectiveTo = &parsed
	}
	if existing, err := s.repo.FindAllowancePolicy(item.ProductCode, item.BillingSubjectType, item.BillingSubjectID, item.AssetCode); err == nil {
		existing.Amount = item.Amount
		existing.ResetCycle = firstNonEmpty(item.ResetCycle, existing.ResetCycle)
		existing.Status = item.Status
		existing.EffectiveFrom = item.EffectiveFrom
		existing.EffectiveTo = item.EffectiveTo
		existing.Metadata = item.Metadata
		existing.UpdatedAt = time.Now()
		return existing, s.repo.SaveAllowancePolicy(existing)
	}
	if err := s.repo.CreateAllowancePolicy(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListAllowancePolicies(productCode, assetCode, status string) ([]models.AllowancePolicy, error) {
	return s.repo.ListAllowancePolicies(productCode, assetCode, status)
}

func (s *Service) GetAllowancePolicy(id string) (*models.AllowancePolicy, error) {
	return s.repo.FindAllowancePolicyByID(id)
}

func (s *Service) UpdateAllowancePolicy(id string, input UpdateAllowancePolicyInput) (*models.AllowancePolicy, error) {
	item, err := s.repo.FindAllowancePolicyByID(id)
	if err != nil {
		return nil, err
	}
	effectiveFrom, effectiveTo, err := parseEffectiveWindow(input.EffectiveFrom, input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	if input.ProductCode != "" {
		item.ProductCode = input.ProductCode
	}
	if input.BillingSubjectType != "" {
		item.BillingSubjectType = input.BillingSubjectType
	}
	if input.BillingSubjectID != "" {
		item.BillingSubjectID = input.BillingSubjectID
	}
	if input.AssetCode != "" {
		item.AssetCode = input.AssetCode
	}
	if input.Amount != nil {
		item.Amount = *input.Amount
	}
	if input.ResetCycle != "" {
		item.ResetCycle = input.ResetCycle
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.EffectiveFrom != "" || input.EffectiveTo != "" {
		item.EffectiveFrom = effectiveFrom
		item.EffectiveTo = effectiveTo
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	return item, s.repo.SaveAllowancePolicy(item)
}

func (s *Service) DeleteAllowancePolicy(id string) (*models.AllowancePolicy, error) {
	item, err := s.repo.FindAllowancePolicyByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteAllowancePolicy(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateWalletAccount(input CreateWalletAccountInput) (*models.WalletAccount, error) {
	log := logger.With(
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"asset_code", input.AssetCode,
		"asset_type", input.AssetType,
	)
	log.Info("wallet.account.create.begin")
	if existing, err := s.repo.FindWalletAccount(input.BillingSubjectType, input.BillingSubjectID, input.AssetCode); err == nil {
		log.Info("wallet.account.create.reused", "wallet_account_id", existing.ID)
		return existing, nil
	}
	item := &models.WalletAccount{
		ID:                 utils.GenerateID(),
		BillingSubjectType: input.BillingSubjectType,
		BillingSubjectID:   input.BillingSubjectID,
		AssetCode:          input.AssetCode,
		AssetType:          input.AssetType,
		Status:             defaultString(input.Status, platformconst.StatusActive),
		Metadata:           input.Metadata,
	}
	if err := s.repo.CreateWalletAccount(item); err != nil {
		log.Error("wallet.account.create.failed", "error", err)
		return nil, err
	}
	log.Info("wallet.account.create.success", "wallet_account_id", item.ID)
	return item, nil
}

func (s *Service) ListWalletAccounts(subjectType, subjectID string) ([]models.WalletAccount, error) {
	return s.repo.ListWalletAccounts(subjectType, subjectID)
}

func (s *Service) ListScopedWalletAccounts(subjectType, subjectID, productCode string, includeAll bool) ([]models.WalletAccount, error) {
	accounts, err := s.repo.ListWalletAccounts(subjectType, subjectID)
	if err != nil || includeAll {
		return accounts, err
	}

	allowedAssets, err := s.listProductAssetCodes(productCode)
	if err != nil {
		return nil, err
	}
	filtered := make([]models.WalletAccount, 0, len(accounts))
	for _, account := range accounts {
		if _, ok := allowedAssets[account.AssetCode]; ok {
			filtered = append(filtered, account)
		}
	}
	return filtered, nil
}

func (s *Service) ListWalletBuckets(walletAccountID string) ([]models.WalletBucket, error) {
	return s.repo.ListWalletBuckets(walletAccountID, "")
}

func (s *Service) GetWalletSummary(subjectType, subjectID, productCode string, now time.Time) (*WalletSummary, error) {
	log := logger.With(
		"billing_subject_type", subjectType,
		"billing_subject_id", subjectID,
		"product_code", productCode,
	)
	log.Info("wallet.summary.begin")
	accounts, err := s.repo.ListWalletAccounts(subjectType, subjectID)
	if err != nil {
		log.Error("wallet.summary.accounts_failed", "error", err)
		return nil, err
	}
	defs, err := s.repo.ListAssetDefinitions(productCode, "", platformconst.StatusActive)
	if err != nil {
		log.Error("wallet.summary.asset_definitions_failed", "error", err)
		return nil, err
	}
	defByCode := make(map[string]models.AssetDefinition, len(defs))
	for _, item := range defs {
		defByCode[item.AssetCode] = item
	}
	out := &WalletSummary{
		BillingSubjectType: subjectType,
		BillingSubjectID:   subjectID,
		ProductCode:        productCode,
		Assets:             make([]WalletAssetSummary, 0),
	}
	for _, account := range accounts {
		def, hasDef := defByCode[account.AssetCode]
		if productCode != "" && hasDef && def.ProductCode != "" && def.ProductCode != productCode {
			continue
		}
		if productCode != "" && !hasDef {
			continue
		}
		buckets, err := s.repo.ListSpendableWalletBuckets(account.ID, now)
		if err != nil {
			log.Error("wallet.summary.spendable_buckets_failed", "error", err, "wallet_account_id", account.ID, "asset_code", account.AssetCode)
			return nil, err
		}
		allBuckets, err := s.repo.ListWalletBuckets(account.ID, "")
		if err != nil {
			log.Error("wallet.summary.all_buckets_failed", "error", err, "wallet_account_id", account.ID, "asset_code", account.AssetCode)
			return nil, err
		}
		summary := WalletAssetSummary{
			AssetCode:      account.AssetCode,
			AssetType:      firstNonEmpty(account.AssetType, def.AssetType),
			LifecycleType:  defaultString(def.LifecycleType, "permanent"),
			AccountBalance: account.Balance,
		}
		for _, bucket := range buckets {
			summary.AvailableBalance += bucket.Balance
			if bucket.ExpiresAt != nil {
				summary.ExpiringBalance += bucket.Balance
				if summary.NextExpiresAt == nil || bucket.ExpiresAt.Before(*summary.NextExpiresAt) {
					expiresAt := *bucket.ExpiresAt
					summary.NextExpiresAt = &expiresAt
				}
			}
		}
		if summary.AvailableBalance == 0 && len(allBuckets) == 0 {
			summary.AvailableBalance = account.Balance
		}
		out.TotalBalance += summary.AvailableBalance
		switch summary.LifecycleType {
		case "cycle_reset":
			out.AllowanceBalance += summary.AvailableBalance
		case "expiring":
			out.RewardBalance += summary.AvailableBalance
		default:
			out.PermanentBalance += summary.AvailableBalance
		}
		out.Assets = append(out.Assets, summary)
	}
	log.Info("wallet.summary.success", "account_count", len(accounts), "asset_count", len(out.Assets), "total_balance", out.TotalBalance)
	return out, nil
}

func (s *Service) GetWalletAccount(id string) (*models.WalletAccount, error) {
	return s.repo.FindWalletAccountByID(id)
}

func (s *Service) PostLedger(input PostWalletLedgerInput) (*models.WalletLedger, *models.WalletAccount, error) {
	log := logger.With(
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"asset_code", input.AssetCode,
		"direction", input.Direction,
		"amount", input.Amount,
	)
	log.Info("wallet.post.begin")

	if input.Amount <= 0 {
		input.Amount = 0
	}
	if input.Direction == platformconst.LedgerDirectionDebit && input.ReferenceType != "" && input.ReferenceID != "" {
		existing, err := s.repo.FindWalletLedgerByReference(input.BillingSubjectType, input.BillingSubjectID, input.AssetCode, input.Direction, input.ReferenceType, input.ReferenceID)
		if err == nil {
			account, accountErr := s.repo.FindWalletAccountByID(existing.WalletAccountID)
			if accountErr != nil {
				return nil, nil, accountErr
			}
			log.Info("wallet.post.idempotent_hit", "wallet_ledger_id", existing.ID)
			return existing, account, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, err
		}
	}
	var account *models.WalletAccount
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		account, err = s.findOrCreateWalletAccountTx(tx, input.BillingSubjectType, input.BillingSubjectID, input.AssetCode, defaultString(input.AssetType, platformconst.WalletAssetTypeCredit))
		if err != nil {
			return err
		}
		switch input.Direction {
		case platformconst.LedgerDirectionCredit:
			_, err = s.creditAccountTx(tx, account, input.Amount, input.ReferenceType, input.ReferenceID, input.Reason, input.Metadata, input.ExpiresAt, input.CycleKey)
			return err
		case platformconst.LedgerDirectionDebit:
			_, err = s.debitAccountTx(tx, account, input.Amount, input.ReferenceType, input.ReferenceID, input.Reason, input.Metadata)
			return err
		default:
			return errors.New("invalid wallet direction")
		}
	})
	if err != nil {
		log.Error("wallet.post.failed", "error", err)
		return nil, nil, err
	}
	items, _ := s.repo.ListWalletLedger(account.ID)
	if len(items) == 0 {
		return nil, account, nil
	}
	log.Info("wallet.post.success", "wallet_account_id", account.ID, "balance", account.Balance)
	return &items[0], account, nil
}

func (s *Service) ListWalletLedger(walletAccountID string) ([]models.WalletLedger, error) {
	return s.repo.ListWalletLedger(walletAccountID)
}

func (s *Service) ListScopedWalletLedger(walletAccountID, productCode string, includeAll bool) ([]models.WalletLedger, error) {
	if includeAll {
		return s.repo.ListWalletLedger(walletAccountID)
	}

	account, err := s.repo.FindWalletAccountByID(walletAccountID)
	if err != nil {
		return nil, err
	}
	if ok, matchErr := s.walletAccountMatchesProduct(account.AssetCode, productCode); matchErr != nil {
		return nil, matchErr
	} else if !ok {
		return []models.WalletLedger{}, nil
	}
	return s.repo.ListWalletLedger(walletAccountID)
}

func (s *Service) GrantCycleAllowance(input GrantCycleAllowanceInput) (*models.WalletBucket, *models.WalletAccount, error) {
	log := logger.With(
		"billing_subject_type", input.BillingSubjectType,
		"billing_subject_id", input.BillingSubjectID,
		"asset_code", input.AssetCode,
		"cycle_key", input.CycleKey,
		"amount", input.Amount,
	)
	log.Info("wallet.allowance.grant.begin")
	var account *models.WalletAccount
	var bucket *models.WalletBucket
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		var err error
		account, err = s.findOrCreateWalletAccountTx(tx, input.BillingSubjectType, input.BillingSubjectID, input.AssetCode, platformconst.WalletAssetTypeSubscriptionAllow)
		if err != nil {
			return err
		}
		bucket, err = s.grantCycleBucketTx(tx, account, input.CycleKey, input.Amount, input.Metadata)
		return err
	})
	if err != nil {
		log.Error("wallet.allowance.grant.failed", "error", err)
		return nil, nil, err
	}
	log.Info("wallet.allowance.grant.success", "wallet_account_id", account.ID, "wallet_bucket_id", bucket.ID)
	return bucket, account, err
}

func (s *Service) listProductAssetCodes(productCode string) (map[string]struct{}, error) {
	defs, err := s.repo.ListAssetDefinitions(productCode, "", "")
	if err != nil {
		return nil, err
	}
	allowedAssets := make(map[string]struct{}, len(defs))
	for _, item := range defs {
		allowedAssets[item.AssetCode] = struct{}{}
	}
	return allowedAssets, nil
}

func (s *Service) walletAccountMatchesProduct(assetCode, productCode string) (bool, error) {
	def, err := s.repo.FindAssetDefinition(assetCode)
	if err != nil {
		return false, err
	}
	return def != nil && def.ProductCode == productCode, nil
}

func (s *Service) ExpireWalletBuckets(assetCode string, now time.Time) ([]models.WalletBucket, error) {
	log := logger.With("asset_code", assetCode, "now", now.Format(time.RFC3339))
	log.Info("wallet.bucket.expire.begin")
	var expired []models.WalletBucket
	err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		// NOTE: 查询和锁定待过期桶必须在事务内，防止并发修改
		var items []models.WalletBucket
		q := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status = ? AND expires_at IS NOT NULL AND expires_at <= ? AND balance > 0", platformconst.StatusActive, now)
		if assetCode != "" {
			q = q.Where("asset_code = ?", assetCode)
		}
		if err := q.Order("expires_at asc, created_at asc").Find(&items).Error; err != nil {
			log.Error("wallet.bucket.expire.lookup_failed", "error", err)
			return err
		}
		if len(items) == 0 {
			log.Info("wallet.bucket.expire.noop")
			return nil
		}
		for i := range items {
			item := &items[i]
			if item.Balance <= 0 {
				item.Status = platformconst.WalletBucketStatusExpired
				item.UpdatedAt = now
				if err := tx.Save(item).Error; err != nil {
					return err
				}
				continue
			}
			var account models.WalletAccount
			if err := tx.Where("id = ?", item.WalletAccountID).First(&account).Error; err != nil {
				return err
			}
			expireAmount := item.Balance
			item.Balance = 0
			item.Status = platformconst.WalletBucketStatusExpired
			item.UpdatedAt = now
			if err := tx.Save(item).Error; err != nil {
				return err
			}
			if account.Balance >= expireAmount {
				account.Balance -= expireAmount
			} else {
				account.Balance = 0
			}
			if err := tx.Save(&account).Error; err != nil {
				return err
			}
			ledger := &models.WalletLedger{
				ID:                 utils.GenerateID(),
				WalletAccountID:    account.ID,
				WalletBucketID:     item.ID,
				BillingSubjectType: account.BillingSubjectType,
				BillingSubjectID:   account.BillingSubjectID,
				AssetCode:          account.AssetCode,
				Direction:          platformconst.LedgerDirectionDebit,
				Amount:             expireAmount,
				Reason:             "asset_expire",
				ReferenceType:      "wallet_bucket",
				ReferenceID:        item.ID,
				Status:             platformconst.WalletLedgerStatusPosted,
				CreatedAt:          now,
			}
			if err := tx.Create(ledger).Error; err != nil {
				return err
			}
		}
		expired = items
		return nil
	})
	if err != nil {
		log.Error("wallet.bucket.expire.failed", "error", err)
		return nil, err
	}
	log.Info("wallet.bucket.expire.success", "expired_bucket_count", len(expired))
	return expired, err
}

func (s *Service) RunLifecycleOnce(now time.Time) (*LifecycleRunResult, error) {
	log := logger.With("now", now.Format(time.RFC3339))
	log.Info("wallet.lifecycle.begin")
	expired, err := s.ExpireWalletBuckets("", now)
	if err != nil {
		log.Error("wallet.lifecycle.expire_failed", "error", err)
		return nil, err
	}
	granted, err := s.RunCycleAllowanceReset("", now)
	if err != nil {
		log.Error("wallet.lifecycle.allowance_reset_failed", "error", err)
		return nil, err
	}
	out := &LifecycleRunResult{
		ExpiredBucketCount: len(expired),
		GrantedPolicyCount: len(granted),
		GrantedBucketIDs:   make([]string, 0, len(granted)),
	}
	for _, item := range granted {
		out.GrantedBucketIDs = append(out.GrantedBucketIDs, item.ID)
	}
	log.Info("wallet.lifecycle.success", "expired_bucket_count", out.ExpiredBucketCount, "granted_policy_count", out.GrantedPolicyCount)
	return out, nil
}

func (s *Service) RunCycleAllowanceReset(productCode string, now time.Time) ([]models.WalletBucket, error) {
	log := logger.With("product_code", productCode, "now", now.Format(time.RFC3339))
	log.Info("wallet.allowance.reset.begin")
	policies, err := s.repo.ListAllowancePolicies(productCode, "", platformconst.StatusActive)
	if err != nil {
		log.Error("wallet.allowance.reset.policy_lookup_failed", "error", err)
		return nil, err
	}
	if len(policies) == 0 {
		log.Info("wallet.allowance.reset.noop")
		return nil, nil
	}
	out := make([]models.WalletBucket, 0)
	err = s.repo.DB().Transaction(func(tx *gorm.DB) error {
		for _, policy := range policies {
			if !allowancePolicyEffective(policy, now) {
				continue
			}
			asset, err := s.resolveAssetDefinitionTx(tx, policy.AssetCode)
			if err != nil {
				return err
			}
			resetCycle := firstNonEmpty(policy.ResetCycle, asset.ResetCycle, "monthly")
			cycleKey := buildCycleKey(resetCycle, now)
			account, err := s.findOrCreateWalletAccountTx(tx, policy.BillingSubjectType, policy.BillingSubjectID, policy.AssetCode, platformconst.WalletAssetTypeSubscriptionAllow)
			if err != nil {
				return err
			}
			bucket, err := s.grantCycleBucketTx(tx, account, cycleKey, policy.Amount, policy.Metadata)
			if err != nil {
				return err
			}
			out = append(out, *bucket)
		}
		return nil
	})
	if err != nil {
		log.Error("wallet.allowance.reset.failed", "error", err, "policy_count", len(policies))
		return nil, err
	}
	log.Info("wallet.allowance.reset.success", "policy_count", len(policies), "granted_bucket_count", len(out))
	return out, err
}

func (s *Service) StartLifecycleScheduler(ctx context.Context, expireInterval, cycleInterval time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	if expireInterval <= 0 {
		expireInterval = time.Hour
	}
	if cycleInterval <= 0 {
		cycleInterval = time.Hour
	}
	go s.runExpireLoop(ctx, expireInterval)
	go s.runCycleLoop(ctx, cycleInterval)
}
