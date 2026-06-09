package wallet

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/logger"
	"platform-service/pkg/platformconst"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) SpendableCreditsBalance(subjectType, subjectID string, now time.Time) (int64, error) {
	accounts, err := s.repo.ListWalletAccounts(subjectType, subjectID)
	if err != nil {
		return 0, err
	}
	var total int64
	for _, account := range accounts {
		if !isCreditAssetType(account.AssetType) {
			continue
		}
		spendable, err := s.accountSpendableBalance(account, now)
		if err != nil {
			return 0, err
		}
		total += spendable
	}
	return total, nil
}

func (s *Service) DebitByPriorityTx(tx *gorm.DB, subjectType, subjectID, productCode, primaryAssetCode string, amount int64, reason, referenceType, referenceID, metadata string) (int64, string, []DebitBreakdown, error) {
	if amount <= 0 {
		return 0, "", nil, nil
	}
	accounts, err := s.prioritizedCreditAccountsTx(tx, subjectType, subjectID, productCode, primaryAssetCode)
	if err != nil {
		return 0, "", nil, err
	}
	remaining := amount
	var totalDebited int64
	var usedCode string
	breakdownByCode := make(map[string]int64, len(accounts))
	for i := range accounts {
		if remaining <= 0 {
			break
		}
		account := &accounts[i]
		if account.Balance <= 0 {
			continue
		}
		useAmount := minInt64(account.Balance, remaining)
		if useAmount <= 0 {
			continue
		}
		if _, err := s.debitAccountTx(tx, account, useAmount, referenceType, referenceID, reason, metadata); err != nil {
			if errors.Is(err, ErrInsufficientWalletBalance) {
				continue
			}
			return totalDebited, usedCode, buildDebitBreakdownSlice(breakdownByCode), err
		}
		totalDebited += useAmount
		remaining -= useAmount
		if usedCode == "" {
			usedCode = account.AssetCode
		}
		breakdownByCode[account.AssetCode] += useAmount
	}
	if remaining > 0 {
		return totalDebited, usedCode, buildDebitBreakdownSlice(breakdownByCode), ErrInsufficientWalletBalance
	}
	return totalDebited, usedCode, buildDebitBreakdownSlice(breakdownByCode), nil
}

func (s *Service) findOrCreateWalletAccountTx(tx *gorm.DB, subjectType, subjectID, assetCode, assetType string) (*models.WalletAccount, error) {
	var account models.WalletAccount
	// NOTE: 使用 FOR UPDATE 行级锁，防止并发事务读到相同余额导致丢失更新
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("billing_subject_type = ? AND billing_subject_id = ? AND asset_code = ?", subjectType, subjectID, assetCode).
		First(&account).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		account = models.WalletAccount{
			ID:                 utils.GenerateID(),
			BillingSubjectType: subjectType,
			BillingSubjectID:   subjectID,
			AssetCode:          assetCode,
			AssetType:          assetType,
			Status:             platformconst.StatusActive,
		}
		if err := tx.Create(&account).Error; err != nil {
			return nil, err
		}
	}
	return &account, nil
}

func (s *Service) resolveAssetDefinitionTx(tx *gorm.DB, assetCode string) (*models.AssetDefinition, error) {
	var asset models.AssetDefinition
	if err := tx.Where("asset_code = ?", assetCode).First(&asset).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &models.AssetDefinition{AssetCode: assetCode, AssetType: platformconst.WalletAssetTypeCredit, LifecycleType: platformconst.WalletLifecyclePermanent, Status: platformconst.StatusActive}, nil
		}
		return nil, err
	}
	return &asset, nil
}

func (s *Service) creditAccountTx(tx *gorm.DB, account *models.WalletAccount, amount int64, referenceType, referenceID, reason, metadata, expiresAtRaw, cycleKey string) (*models.WalletLedger, error) {
	if amount <= 0 {
		return nil, nil
	}
	asset, err := s.resolveAssetDefinitionTx(tx, account.AssetCode)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	bucket := &models.WalletBucket{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          firstNonEmpty(asset.AssetType, account.AssetType),
		LifecycleType:      defaultString(asset.LifecycleType, platformconst.WalletLifecyclePermanent),
		SourceType:         referenceType,
		SourceID:           referenceID,
		CycleKey:           cycleKey,
		Balance:            amount,
		Status:             platformconst.StatusActive,
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if bucket.LifecycleType == platformconst.WalletLifecycleExpiring {
		if expiresAtRaw != "" {
			parsed, err := time.Parse(time.RFC3339, expiresAtRaw)
			if err != nil {
				return nil, err
			}
			bucket.ExpiresAt = &parsed
		} else if asset.DefaultExpireDays > 0 {
			expiresAt := now.AddDate(0, 0, asset.DefaultExpireDays)
			bucket.ExpiresAt = &expiresAt
		}
	}
	if bucket.LifecycleType == platformconst.WalletLifecycleCycleReset && bucket.CycleKey != "" {
		// NOTE: cycle bucket 查询必须使用 tx 保持事务一致性
		var existingBucket models.WalletBucket
		cycleFindErr := tx.Where("wallet_account_id = ? AND cycle_key = ?", account.ID, bucket.CycleKey).Order("created_at desc").First(&existingBucket).Error
		if existing := &existingBucket; cycleFindErr == nil {
			existing.Balance += amount
			existing.UpdatedAt = now
			if err := tx.Save(existing).Error; err != nil {
				return nil, err
			}
			bucket = existing
		} else if !errors.Is(cycleFindErr, gorm.ErrRecordNotFound) {
			return nil, cycleFindErr
		} else if err := tx.Create(bucket).Error; err != nil {
			return nil, err
		}
	} else if err := tx.Create(bucket).Error; err != nil {
		return nil, err
	}
	ledger := &models.WalletLedger{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		WalletBucketID:     bucket.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		Direction:          platformconst.LedgerDirectionCredit,
		Amount:             amount,
		Reason:             reason,
		ReferenceType:      referenceType,
		ReferenceID:        referenceID,
		Status:             platformconst.WalletLedgerStatusPosted,
		Metadata:           metadata,
		CreatedAt:          now,
	}
	if err := tx.Create(ledger).Error; err != nil {
		return nil, err
	}
	account.Balance += amount
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	return ledger, nil
}

func (s *Service) debitAccountTx(tx *gorm.DB, account *models.WalletAccount, amount int64, referenceType, referenceID, reason, metadata string) (*models.WalletLedger, error) {
	if amount <= 0 {
		return nil, nil
	}
	if account.Balance < amount {
		return nil, ErrInsufficientWalletBalance
	}
	now := time.Now()
	// NOTE: bucket 查询必须使用事务内的 tx 而非 s.repo，
	// 否则并发事务可读到相同余额导致双花。
	var buckets []models.WalletBucket
	if err := tx.
		Where("wallet_account_id = ? AND status = ? AND balance > 0", account.ID, platformconst.StatusActive).
		Where("expires_at IS NULL OR expires_at > ?", now).
		Order("CASE WHEN expires_at IS NULL THEN 1 ELSE 0 END, expires_at asc, created_at asc").
		Find(&buckets).Error; err != nil {
		return nil, err
	}
	var allBuckets []models.WalletBucket
	if err := tx.
		Where("wallet_account_id = ?", account.ID).
		Order("expires_at asc, created_at asc").
		Find(&allBuckets).Error; err != nil {
		return nil, err
	}
	asset, err := s.resolveAssetDefinitionTx(tx, account.AssetCode)
	if err != nil {
		return nil, err
	}
	remaining := amount
	var lastLedger *models.WalletLedger
	for i := range buckets {
		if remaining <= 0 {
			break
		}
		bucket := &buckets[i]
		useAmount := minInt64(bucket.Balance, remaining)
		if useAmount <= 0 {
			continue
		}
		ledger := &models.WalletLedger{
			ID:                 utils.GenerateID(),
			WalletAccountID:    account.ID,
			WalletBucketID:     bucket.ID,
			BillingSubjectType: account.BillingSubjectType,
			BillingSubjectID:   account.BillingSubjectID,
			AssetCode:          account.AssetCode,
			Direction:          "debit",
			Amount:             useAmount,
			Reason:             reason,
			ReferenceType:      referenceType,
			ReferenceID:        referenceID,
			Status:             "posted",
			Metadata:           metadata,
			CreatedAt:          now,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return nil, err
		}
		bucket.Balance -= useAmount
		if bucket.Balance == 0 {
			bucket.Status = "consumed"
		}
		bucket.UpdatedAt = now
		if err := tx.Save(bucket).Error; err != nil {
			return nil, err
		}
		remaining -= useAmount
		lastLedger = ledger
	}
	if remaining > 0 && shouldAllowLegacyFallback(asset, allBuckets) {
		legacyLedger := &models.WalletLedger{
			ID:                 utils.GenerateID(),
			WalletAccountID:    account.ID,
			BillingSubjectType: account.BillingSubjectType,
			BillingSubjectID:   account.BillingSubjectID,
			AssetCode:          account.AssetCode,
			Direction:          "debit",
			Amount:             remaining,
			Reason:             reason,
			ReferenceType:      referenceType,
			ReferenceID:        referenceID,
			Status:             "posted",
			Metadata:           metadata,
			CreatedAt:          now,
		}
		if err := tx.Create(legacyLedger).Error; err != nil {
			return nil, err
		}
		lastLedger = legacyLedger
	}
	if remaining > 0 && !shouldAllowLegacyFallback(asset, allBuckets) {
		return nil, ErrInsufficientWalletBalance
	}
	account.Balance -= amount
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	return lastLedger, nil
}

func (s *Service) grantCycleBucketTx(tx *gorm.DB, account *models.WalletAccount, cycleKey string, amount int64, metadata string) (*models.WalletBucket, error) {
	now := time.Now()
	// NOTE: cycle bucket 查询必须使用 tx 保持事务一致性
	var existingBucket models.WalletBucket
	if err := tx.Where("wallet_account_id = ? AND cycle_key = ?", account.ID, cycleKey).Order("created_at desc").First(&existingBucket).Error; err == nil {
		return &existingBucket, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	bucket := &models.WalletBucket{
		ID:                 utils.GenerateID(),
		WalletAccountID:    account.ID,
		BillingSubjectType: account.BillingSubjectType,
		BillingSubjectID:   account.BillingSubjectID,
		AssetCode:          account.AssetCode,
		AssetType:          "subscription_allowance",
		LifecycleType:      "cycle_reset",
		CycleKey:           cycleKey,
		Balance:            amount,
		Status:             "active",
		Metadata:           metadata,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := tx.Create(bucket).Error; err != nil {
		return nil, err
	}
	account.Balance += amount
	if err := tx.Save(account).Error; err != nil {
		return nil, err
	}
	return bucket, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func parseEffectiveWindow(from, to string) (*time.Time, *time.Time, error) {
	var (
		fromPtr *time.Time
		toPtr   *time.Time
	)
	if from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return nil, nil, err
		}
		fromPtr = &parsed
	}
	if to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return nil, nil, err
		}
		toPtr = &parsed
	}
	return fromPtr, toPtr, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func minInt64(a, b int64) int64 {
	if a <= b {
		return a
	}
	return b
}

func shouldAllowLegacyFallback(asset *models.AssetDefinition, allBuckets []models.WalletBucket) bool {
	if len(allBuckets) > 0 {
		return false
	}
	if asset == nil {
		return true
	}
	return defaultString(asset.LifecycleType, "permanent") == "permanent"
}

func (s *Service) accountSpendableBalance(account models.WalletAccount, now time.Time) (int64, error) {
	buckets, err := s.repo.ListSpendableWalletBuckets(account.ID, now)
	if err != nil {
		return 0, err
	}
	var balance int64
	for _, bucket := range buckets {
		balance += bucket.Balance
	}
	if balance > 0 {
		return balance, nil
	}
	allBuckets, err := s.repo.ListWalletBuckets(account.ID, "")
	if err != nil {
		return 0, err
	}
	asset, err := s.resolveAssetDefinitionTx(s.repo.DB(), account.AssetCode)
	if err != nil {
		return 0, err
	}
	if shouldAllowLegacyFallback(asset, allBuckets) {
		return account.Balance, nil
	}
	return 0, nil
}

func (s *Service) prioritizedCreditAccountsTx(tx *gorm.DB, subjectType, subjectID, productCode, primaryAssetCode string) ([]models.WalletAccount, error) {
	if tx == nil {
		tx = s.repo.DB()
	}
	var accounts []models.WalletAccount
	err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("billing_subject_type = ? AND billing_subject_id = ?", subjectType, subjectID).
		Order("created_at asc").
		Find(&accounts).Error
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, nil
	}
	accountByCode := make(map[string]models.WalletAccount, len(accounts))
	for _, account := range accounts {
		if !isCreditAssetType(account.AssetType) {
			continue
		}
		accountByCode[account.AssetCode] = account
	}
	if len(accountByCode) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	orderedCodes := make([]string, 0, len(accountByCode))
	if productCode != "" {
		var defs []models.AssetDefinition
		err := tx.Where("product_code = ? AND status = ?", productCode, platformconst.StatusActive).
			Order("asset_code asc").
			Find(&defs).Error
		if err != nil {
			return nil, err
		}
		sort.SliceStable(defs, func(i, j int) bool {
			return lifecyclePriority(defs[i].LifecycleType) < lifecyclePriority(defs[j].LifecycleType)
		})
		for _, def := range defs {
			if _, ok := accountByCode[def.AssetCode]; !ok {
				continue
			}
			if _, ok := seen[def.AssetCode]; ok {
				continue
			}
			orderedCodes = append(orderedCodes, def.AssetCode)
			seen[def.AssetCode] = struct{}{}
		}
	}
	if primaryAssetCode != "" {
		if _, ok := accountByCode[primaryAssetCode]; ok {
			if _, exists := seen[primaryAssetCode]; !exists {
				orderedCodes = append(orderedCodes, primaryAssetCode)
				seen[primaryAssetCode] = struct{}{}
			}
		}
	}
	remaining := make([]models.WalletAccount, 0, len(accountByCode))
	for _, account := range accountByCode {
		if _, ok := seen[account.AssetCode]; ok {
			continue
		}
		remaining = append(remaining, account)
	}
	sort.SliceStable(remaining, func(i, j int) bool {
		left := creditAssetTypePriority(remaining[i].AssetType)
		right := creditAssetTypePriority(remaining[j].AssetType)
		if left == right {
			if remaining[i].CreatedAt.Equal(remaining[j].CreatedAt) {
				return remaining[i].AssetCode < remaining[j].AssetCode
			}
			return remaining[i].CreatedAt.Before(remaining[j].CreatedAt)
		}
		return left < right
	})
	ordered := make([]models.WalletAccount, 0, len(accountByCode))
	for _, code := range orderedCodes {
		ordered = append(ordered, accountByCode[code])
	}
	ordered = append(ordered, remaining...)
	return ordered, nil
}

func lifecyclePriority(value string) int {
	switch value {
	case platformconst.WalletLifecycleCycleReset:
		return 0
	case platformconst.WalletLifecycleExpiring:
		return 1
	default:
		return 2
	}
}

func creditAssetTypePriority(value string) int {
	switch value {
	case platformconst.WalletAssetTypeSubscriptionAllow:
		return 0
	case platformconst.WalletAssetTypeRewardCredit:
		return 1
	case platformconst.WalletAssetTypeCredit:
		return 2
	default:
		return 3
	}
}

func isCreditAssetType(value string) bool {
	switch value {
	case "", platformconst.WalletAssetTypeCredit, platformconst.WalletAssetTypeRewardCredit, platformconst.WalletAssetTypeSubscriptionAllow:
		return true
	default:
		return false
	}
}

func buildDebitBreakdownSlice(byCode map[string]int64) []DebitBreakdown {
	if len(byCode) == 0 {
		return nil
	}
	codes := make([]string, 0, len(byCode))
	for code := range byCode {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]DebitBreakdown, 0, len(codes))
	for _, code := range codes {
		out = append(out, DebitBreakdown{AssetCode: code, Amount: byCode[code]})
	}
	return out
}

func allowancePolicyEffective(item models.AllowancePolicy, now time.Time) bool {
	if item.EffectiveFrom != nil && now.Before(*item.EffectiveFrom) {
		return false
	}
	if item.EffectiveTo != nil && now.After(*item.EffectiveTo) {
		return false
	}
	return true
}

func buildCycleKey(resetCycle string, now time.Time) string {
	switch resetCycle {
	case "daily":
		return now.UTC().Format("2006-01-02")
	case "weekly":
		year, week := now.UTC().ISOWeek()
		return fmt.Sprintf("%04d-W%02d", year, week)
	default:
		return now.UTC().Format("2006-01")
	}
}

func (s *Service) runExpireLoop(ctx context.Context, interval time.Duration) {
	if _, err := s.ExpireWalletBuckets("", time.Now()); err != nil {
		logger.With("error", err).Error("wallet.lifecycle.expire.initial_failed")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.ExpireWalletBuckets("", time.Now()); err != nil {
				logger.With("error", err).Error("wallet.lifecycle.expire.failed")
			}
		}
	}
}

func (s *Service) runCycleLoop(ctx context.Context, interval time.Duration) {
	if _, err := s.RunCycleAllowanceReset("", time.Now()); err != nil {
		logger.With("error", err).Error("wallet.lifecycle.cycle_reset.initial_failed")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.RunCycleAllowanceReset("", time.Now()); err != nil {
				logger.With("error", err).Error("wallet.lifecycle.cycle_reset.failed")
			}
		}
	}
}
