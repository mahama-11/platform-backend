package incentive

import "testing"

func TestRewardCommissionAndReferralCrud(t *testing.T) {
	service := newReferralTestService(t)

	reward, err := service.CreateReward(CreateRewardInput{
		ProductCode:            "ecommerce",
		RewardType:             "campaign_reward",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-1",
		AssetCode:              "ECOM_CREDIT",
		Amount:                 30,
		Status:                 "issued",
		ReferenceType:          "campaign",
		ReferenceID:            "camp-1",
	})
	if err != nil {
		t.Fatalf("CreateReward: %v", err)
	}
	if _, err := service.UpdateReward(reward.ID, UpdateRewardInput{Status: "redeemed"}); err != nil {
		t.Fatalf("UpdateReward: %v", err)
	}
	if rewards, err := service.ListRewardLedgers("ecommerce", "organization", "org-1"); err != nil || len(rewards) == 0 {
		t.Fatalf("ListRewardLedgers: %+v err=%v", rewards, err)
	}
	if _, err := service.GetRewardLedger(reward.ID); err != nil {
		t.Fatalf("GetRewardLedger: %v", err)
	}

	commission, err := service.CreateCommission(CreateCommissionInput{
		ProductCode:            "ecommerce",
		CommissionType:         "channel_referral",
		BeneficiarySubjectType: "organization",
		BeneficiarySubjectID:   "org-1",
		Amount:                 88,
		Status:                 "pending",
		ReferenceType:          "order",
		ReferenceID:            "order-1",
	})
	if err != nil {
		t.Fatalf("CreateCommission: %v", err)
	}
	if _, err := service.UpdateCommission(commission.ID, UpdateCommissionInput{Status: "earned"}); err != nil {
		t.Fatalf("UpdateCommission: %v", err)
	}
	if commissions, err := service.ListCommissionLedgers("ecommerce", "organization", "org-1", "earned"); err != nil || len(commissions) == 0 {
		t.Fatalf("ListCommissionLedgers: %+v err=%v", commissions, err)
	}
	if _, err := service.GetCommissionLedger(commission.ID); err != nil {
		t.Fatalf("GetCommissionLedger: %v", err)
	}

	program, err := service.CreateReferralProgram(CreateReferralProgramInput{
		ProductCode:           "ecommerce",
		ProgramCode:           "signup-program",
		Name:                  "Signup Program",
		TriggerType:           "signup",
		CommissionPolicy:      "fixed_amount",
		CommissionCurrency:    "ECOM_CREDIT",
		CommissionFixedAmount: 20,
		Status:                "active",
	})
	if err != nil {
		t.Fatalf("CreateReferralProgram: %v", err)
	}
	if _, err := service.UpdateReferralProgram(program.ID, UpdateReferralProgramInput{Name: "Signup Program v2"}); err != nil {
		t.Fatalf("UpdateReferralProgram: %v", err)
	}
	if programs, err := service.ListReferralPrograms("ecommerce", "active"); err != nil || len(programs) == 0 {
		t.Fatalf("ListReferralPrograms: %+v err=%v", programs, err)
	}
	if _, err := service.GetReferralProgram(program.ID); err != nil {
		t.Fatalf("GetReferralProgram: %v", err)
	}

	code, err := service.CreateReferralCode(CreateReferralCodeInput{
		ProgramCode:         "signup-program",
		PromoterSubjectType: "organization",
		PromoterSubjectID:   "org-promoter",
		Status:              "active",
	})
	if err != nil {
		t.Fatalf("CreateReferralCode: %v", err)
	}
	if _, err := service.UpdateReferralCode(code.Code, UpdateReferralCodeInput{Status: "disabled"}); err != nil {
		t.Fatalf("UpdateReferralCode: %v", err)
	}
	if codes, err := service.ListReferralCodes("", "organization", "org-promoter", ""); err != nil || len(codes) == 0 {
		t.Fatalf("ListReferralCodes: %+v err=%v", codes, err)
	}
	if _, err := service.GetReferralCode(code.Code); err != nil {
		t.Fatalf("GetReferralCode: %v", err)
	}
}
