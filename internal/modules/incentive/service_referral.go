package incentive

import (
	"errors"
	"strings"
	"time"

	"platform-service/internal/models"
	"platform-service/pkg/utils"

	"gorm.io/gorm"
)

func (s *Service) CreateReferralProgram(input CreateReferralProgramInput) (*models.ReferralProgram, error) {
	commissionPolicy := defaultString(input.CommissionPolicy, "fixed_amount")
	if err := validateCommissionPolicy(commissionPolicy, input.CommissionFixedAmount, input.CommissionRateBps); err != nil {
		return nil, err
	}
	effectiveFrom, err := parseOptionalTime(input.EffectiveFrom)
	if err != nil {
		return nil, err
	}
	effectiveTo, err := parseOptionalTime(input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	item := &models.ReferralProgram{
		ID:                    utils.GenerateID(),
		ProductCode:           input.ProductCode,
		ProgramCode:           strings.ToLower(input.ProgramCode),
		Name:                  input.Name,
		Status:                defaultString(input.Status, "active"),
		TriggerType:           input.TriggerType,
		CommissionPolicy:      commissionPolicy,
		CommissionCurrency:    defaultString(input.CommissionCurrency, "CNY"),
		CommissionFixedAmount: input.CommissionFixedAmount,
		CommissionRateBps:     input.CommissionRateBps,
		SettlementDelayDays:   input.SettlementDelayDays,
		AllowRepeat:           input.AllowRepeat,
		EffectiveFrom:         effectiveFrom,
		EffectiveTo:           effectiveTo,
		Metadata:              input.Metadata,
	}
	if err := s.repo.CreateReferralProgram(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralPrograms(productCode, status string) ([]models.ReferralProgram, error) {
	return s.repo.ListReferralPrograms(productCode, status)
}

func (s *Service) GetReferralProgram(id string) (*models.ReferralProgram, error) {
	return s.repo.FindReferralProgramByID(id)
}

func (s *Service) UpdateReferralProgram(id string, input UpdateReferralProgramInput) (*models.ReferralProgram, error) {
	item, err := s.repo.FindReferralProgramByID(id)
	if err != nil {
		return nil, err
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.CommissionPolicy != "" {
		item.CommissionPolicy = input.CommissionPolicy
	}
	if input.CommissionCurrency != "" {
		item.CommissionCurrency = input.CommissionCurrency
	}
	if input.CommissionFixedAmount > 0 {
		item.CommissionFixedAmount = input.CommissionFixedAmount
	}
	if input.CommissionRateBps > 0 {
		item.CommissionRateBps = input.CommissionRateBps
	}
	if input.SettlementDelayDays > 0 {
		item.SettlementDelayDays = input.SettlementDelayDays
	}
	if input.AllowRepeat != nil {
		item.AllowRepeat = *input.AllowRepeat
	}
	if input.EffectiveFrom != "" {
		value, parseErr := parseOptionalTime(input.EffectiveFrom)
		if parseErr != nil {
			return nil, parseErr
		}
		item.EffectiveFrom = value
	}
	if input.EffectiveTo != "" {
		value, parseErr := parseOptionalTime(input.EffectiveTo)
		if parseErr != nil {
			return nil, parseErr
		}
		item.EffectiveTo = value
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := validateCommissionPolicy(item.CommissionPolicy, item.CommissionFixedAmount, item.CommissionRateBps); err != nil {
		return nil, err
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralProgram(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateReferralCode(input CreateReferralCodeInput) (*models.ReferralCode, error) {
	program, err := s.repo.FindReferralProgramByCode(strings.ToLower(input.ProgramCode))
	if err != nil {
		return nil, err
	}
	item := &models.ReferralCode{
		ID:                  utils.GenerateID(),
		ProgramID:           program.ID,
		ProductCode:         program.ProductCode,
		Code:                normalizeReferralCode(input.Code),
		PromoterSubjectType: input.PromoterSubjectType,
		PromoterSubjectID:   input.PromoterSubjectID,
		Status:              defaultString(input.Status, "active"),
		Metadata:            input.Metadata,
	}
	if item.Code == "" {
		item.Code = generateReferralCode()
	}
	if err := s.repo.CreateReferralCode(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralCodes(programID, promoterType, promoterID, status string) ([]models.ReferralCode, error) {
	return s.repo.ListReferralCodes(programID, promoterType, promoterID, status)
}

func (s *Service) GetReferralCode(code string) (*models.ReferralCode, error) {
	return s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
}

func (s *Service) ResolveReferralCode(code, productCode string) (*ResolvedReferralCode, error) {
	item, err := s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReferralCodeNotFound
		}
		return nil, err
	}
	if item.Status != "active" {
		return nil, ErrReferralCodeInactive
	}
	program, err := s.repo.FindReferralProgramByID(item.ProgramID)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if program.Status != "active" || !programEffectiveNow(*program, now) {
		return nil, ErrReferralProgramInactive
	}
	if productCode != "" && program.ProductCode != productCode {
		return nil, ErrReferralProductMismatch
	}
	return &ResolvedReferralCode{
		Code:                  item.Code,
		ProductCode:           program.ProductCode,
		ProgramID:             program.ID,
		ProgramCode:           program.ProgramCode,
		ProgramName:           program.Name,
		TriggerType:           program.TriggerType,
		CommissionPolicy:      program.CommissionPolicy,
		CommissionCurrency:    program.CommissionCurrency,
		CommissionFixedAmount: program.CommissionFixedAmount,
		CommissionRateBps:     program.CommissionRateBps,
		SettlementDelayDays:   program.SettlementDelayDays,
		AllowRepeat:           program.AllowRepeat,
		RewardPolicyDesc:      buildRewardPolicyDesc(*program),
		PromoterSubjectType:   item.PromoterSubjectType,
		PromoterSubjectID:     item.PromoterSubjectID,
		Status:                item.Status,
		Metadata:              parseMetadata(item.Metadata),
	}, nil
}

func (s *Service) UpdateReferralCode(code string, input UpdateReferralCodeInput) (*models.ReferralCode, error) {
	item, err := s.repo.FindReferralCodeByCode(normalizeReferralCode(code))
	if err != nil {
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralCode(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateReferralConversion(input CreateReferralConversionInput) (*models.ReferralConversion, error) {
	referralCode := normalizeReferralCode(input.ReferralCode)
	item := &models.ReferralConversion{}
	if err := s.repo.DB().Transaction(func(tx *gorm.DB) error {
		if _, err := s.repo.FindReferralConversionByReference(input.ReferenceType, input.ReferenceID); err == nil {
			return ErrReferralConversionExists
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var code models.ReferralCode
		if err := tx.Where("code = ?", referralCode).First(&code).Error; err != nil {
			return err
		}
		if code.Status != "active" {
			return ErrReferralCodeInactive
		}
		var program models.ReferralProgram
		if err := tx.Where("id = ?", code.ProgramID).First(&program).Error; err != nil {
			return err
		}
		if program.Status != "active" || !programEffectiveNow(program, time.Now()) {
			return ErrReferralProgramInactive
		}
		if program.ProductCode != input.ProductCode {
			return ErrReferralProductMismatch
		}
		if program.TriggerType != input.TriggerType {
			return ErrReferralTriggerMismatch
		}
		if !program.AllowRepeat {
			if _, err := s.repo.FindReferralConversionByTriggerAndSubject(input.ProductCode, input.TriggerType, input.ReferredSubjectType, input.ReferredSubjectID); err == nil {
				return ErrReferralAlreadyClaimed
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		if code.PromoterSubjectType == input.ReferredSubjectType && code.PromoterSubjectID == input.ReferredSubjectID {
			return ErrReferralSelfInviteBlocked
		}

		commissionAmount := calculateCommissionAmount(program, input.CommissionBaseAmount)
		commissionCurrency := defaultString(input.CommissionCurrency, program.CommissionCurrency)
		conversionStatus := deriveReferralConversionStatus(program, commissionAmount)
		*item = models.ReferralConversion{
			ID:                    utils.GenerateID(),
			ProgramID:             program.ID,
			ReferralCodeID:        code.ID,
			ProductCode:           input.ProductCode,
			TriggerType:           input.TriggerType,
			PromoterSubjectType:   code.PromoterSubjectType,
			PromoterSubjectID:     code.PromoterSubjectID,
			ReferredSubjectType:   input.ReferredSubjectType,
			ReferredSubjectID:     input.ReferredSubjectID,
			SettlementSubjectType: input.SettlementSubjectType,
			SettlementSubjectID:   input.SettlementSubjectID,
			ReferenceType:         input.ReferenceType,
			ReferenceID:           input.ReferenceID,
			CommissionCurrency:    commissionCurrency,
			CommissionAmount:      commissionAmount,
			Status:                conversionStatus,
			Metadata:              input.Metadata,
		}
		if err := tx.Create(item).Error; err != nil {
			return err
		}

		if commissionAmount <= 0 {
			return nil
		}

		ledger := &models.CommissionLedger{
			ID:                     utils.GenerateID(),
			ProductCode:            input.ProductCode,
			CommissionType:         program.TriggerType,
			BeneficiarySubjectType: code.PromoterSubjectType,
			BeneficiarySubjectID:   code.PromoterSubjectID,
			SettlementSubjectType:  input.SettlementSubjectType,
			SettlementSubjectID:    input.SettlementSubjectID,
			Currency:               commissionCurrency,
			Amount:                 commissionAmount,
			Status:                 deriveCommissionStatus(program, commissionAmount),
			ReferenceType:          input.ReferenceType,
			ReferenceID:            input.ReferenceID,
			Metadata:               input.Metadata,
		}
		if err := tx.Create(ledger).Error; err != nil {
			return err
		}
		item.CommissionLedgerID = ledger.ID
		item.Status = conversionStatus
		item.UpdatedAt = time.Now()
		return tx.Save(item).Error
	}); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListReferralConversions(productCode, promoterType, promoterID, status string) ([]models.ReferralConversion, error) {
	return s.repo.ListReferralConversions(productCode, promoterType, promoterID, status)
}

func (s *Service) GetReferralConversion(id string) (*models.ReferralConversion, error) {
	return s.repo.FindReferralConversionByID(id)
}

func (s *Service) UpdateReferralConversion(id string, input UpdateReferralConversionInput) (*models.ReferralConversion, error) {
	item, err := s.repo.FindReferralConversionByID(id)
	if err != nil {
		return nil, err
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	item.UpdatedAt = time.Now()
	if err := s.repo.SaveReferralConversion(item); err != nil {
		return nil, err
	}
	return item, nil
}
