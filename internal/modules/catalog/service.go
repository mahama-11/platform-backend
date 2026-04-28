package catalog

import (
	"time"

	"platform-service/internal/models"
	"platform-service/internal/repository"
	"platform-service/pkg/utils"
)

type Service struct {
	repo *repository.CommercialRepository
}

type CreateProductInput struct {
	Code      string `json:"code" binding:"required"`
	Name      string `json:"name" binding:"required"`
	Status    string `json:"status"`
	OwnerTeam string `json:"owner_team"`
	Metadata  string `json:"metadata"`
}

type UpdateProductInput struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	OwnerTeam string `json:"owner_team"`
	Metadata  string `json:"metadata"`
}

type CreateSKUInput struct {
	ProductID   string `json:"product_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	SKUType     string `json:"sku_type" binding:"required"`
	BillingMode string `json:"billing_mode" binding:"required"`
	Currency    string `json:"currency"`
	ListPrice   int64  `json:"list_price"`
	Status      string `json:"status"`
	Metadata    string `json:"metadata"`
}

type UpdateSKUInput struct {
	ProductID   string `json:"product_id"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	SKUType     string `json:"sku_type"`
	BillingMode string `json:"billing_mode"`
	Currency    string `json:"currency"`
	ListPrice   *int64  `json:"list_price"`
	Status      string  `json:"status"`
	Metadata    string  `json:"metadata"`
}

type CreateBillableItemInput struct {
	ProductID       string `json:"product_id" binding:"required"`
	Code            string `json:"code" binding:"required"`
	Name            string `json:"name" binding:"required"`
	MeterUnit       string `json:"meter_unit" binding:"required"`
	BillingScope    string `json:"billing_scope" binding:"required"`
	SettlementMode  string `json:"settlement_mode" binding:"required"`
	PricingBehavior string `json:"pricing_behavior" binding:"required"`
	Status          string `json:"status"`
	Metadata        string `json:"metadata"`
}

type UpdateBillableItemInput struct {
	ProductID       string `json:"product_id"`
	Code            string `json:"code"`
	Name            string `json:"name"`
	MeterUnit       string `json:"meter_unit"`
	BillingScope    string `json:"billing_scope"`
	SettlementMode  string `json:"settlement_mode"`
	PricingBehavior string `json:"pricing_behavior"`
	Status          string `json:"status"`
	Metadata        string `json:"metadata"`
}

type CreatePackageInput struct {
	ProductID   string `json:"product_id" binding:"required"`
	Code        string `json:"code" binding:"required"`
	Name        string `json:"name" binding:"required"`
	PackageType string `json:"package_type" binding:"required"`
	Status      string `json:"status"`
	Metadata    string `json:"metadata"`
}

type UpdatePackageInput struct {
	Name        string `json:"name"`
	PackageType string `json:"package_type"`
	Status      string `json:"status"`
	Metadata    string `json:"metadata"`
}

type CreateRateCardInput struct {
	ProductID     string `json:"product_id" binding:"required"`
	Code          string `json:"code" binding:"required"`
	TargetType    string `json:"target_type" binding:"required"`
	TargetID      string `json:"target_id" binding:"required"`
	PriceModel    string `json:"price_model" binding:"required"`
	Currency      string `json:"currency"`
	PriceConfig   string `json:"price_config"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
	Version       int    `json:"version"`
	Status        string `json:"status"`
	Metadata      string `json:"metadata"`
}

type UpdateRateCardInput struct {
	PriceModel    string `json:"price_model"`
	Currency      string `json:"currency"`
	PriceConfig   string `json:"price_config"`
	EffectiveFrom string `json:"effective_from"`
	EffectiveTo   string `json:"effective_to"`
	Version       int    `json:"version"`
	Status        string `json:"status"`
	Metadata      string `json:"metadata"`
}

type OfferingsView struct {
	Product           *models.Product            `json:"product"`
	SKUs              []models.SKU               `json:"skus"`
	Packages          []models.CommercialPackage `json:"packages"`
	BillableItems     []models.BillableItem      `json:"billable_items"`
	RateCards         []models.RateCard          `json:"rate_cards"`
	AssetDefinitions  []models.AssetDefinition   `json:"asset_definitions"`
	AllowancePolicies []models.AllowancePolicy   `json:"allowance_policies"`
}

func NewService(repo *repository.CommercialRepository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateProduct(input CreateProductInput) (*models.Product, error) {
	item := &models.Product{
		ID:        utils.GenerateID(),
		Code:      input.Code,
		Name:      input.Name,
		Status:    defaultString(input.Status, "active"),
		OwnerTeam: input.OwnerTeam,
		Metadata:  input.Metadata,
	}
	if err := s.repo.CreateProduct(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListProducts() ([]models.Product, error) {
	return s.repo.ListProducts()
}

func (s *Service) GetProduct(id string) (*models.Product, error) {
	return s.repo.FindProductByID(id)
}

func (s *Service) UpdateProduct(id string, input UpdateProductInput) (*models.Product, error) {
	item, err := s.repo.FindProductByID(id)
	if err != nil {
		return nil, err
	}
	if input.Code != "" {
		item.Code = input.Code
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.OwnerTeam != "" {
		item.OwnerTeam = input.OwnerTeam
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SaveProduct(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteProduct(id string) (*models.Product, error) {
	item, err := s.repo.FindProductByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteProduct(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateSKU(input CreateSKUInput) (*models.SKU, error) {
	item := &models.SKU{
		ID:          utils.GenerateID(),
		ProductID:   input.ProductID,
		Code:        input.Code,
		Name:        input.Name,
		SKUType:     input.SKUType,
		BillingMode: input.BillingMode,
		Currency:    defaultString(input.Currency, "CNY"),
		ListPrice:   input.ListPrice,
		Status:      defaultString(input.Status, "active"),
		Metadata:    input.Metadata,
	}
	if err := s.repo.CreateSKU(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListSKUs(productID string) ([]models.SKU, error) {
	return s.repo.ListSKUs(productID)
}

func (s *Service) GetSKU(id string) (*models.SKU, error) {
	return s.repo.FindSKUByID(id)
}

func (s *Service) UpdateSKU(id string, input UpdateSKUInput) (*models.SKU, error) {
	item, err := s.repo.FindSKUByID(id)
	if err != nil {
		return nil, err
	}
	if input.ProductID != "" {
		item.ProductID = input.ProductID
	}
	if input.Code != "" {
		item.Code = input.Code
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.SKUType != "" {
		item.SKUType = input.SKUType
	}
	if input.BillingMode != "" {
		item.BillingMode = input.BillingMode
	}
	if input.Currency != "" {
		item.Currency = input.Currency
	}
	if input.ListPrice != nil {
		item.ListPrice = *input.ListPrice
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SaveSKU(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteSKU(id string) (*models.SKU, error) {
	item, err := s.repo.FindSKUByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteSKU(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateBillableItem(input CreateBillableItemInput) (*models.BillableItem, error) {
	item := &models.BillableItem{
		ID:              utils.GenerateID(),
		ProductID:       input.ProductID,
		Code:            input.Code,
		Name:            input.Name,
		MeterUnit:       input.MeterUnit,
		BillingScope:    input.BillingScope,
		SettlementMode:  input.SettlementMode,
		PricingBehavior: input.PricingBehavior,
		Status:          defaultString(input.Status, "active"),
		Metadata:        input.Metadata,
	}
	if err := s.repo.CreateBillableItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListBillableItems(productID string) ([]models.BillableItem, error) {
	return s.repo.ListBillableItems(productID)
}

func (s *Service) GetBillableItem(id string) (*models.BillableItem, error) {
	return s.repo.FindBillableItemByID(id)
}

func (s *Service) UpdateBillableItem(id string, input UpdateBillableItemInput) (*models.BillableItem, error) {
	item, err := s.repo.FindBillableItemByID(id)
	if err != nil {
		return nil, err
	}
	if input.ProductID != "" {
		item.ProductID = input.ProductID
	}
	if input.Code != "" {
		item.Code = input.Code
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.MeterUnit != "" {
		item.MeterUnit = input.MeterUnit
	}
	if input.BillingScope != "" {
		item.BillingScope = input.BillingScope
	}
	if input.SettlementMode != "" {
		item.SettlementMode = input.SettlementMode
	}
	if input.PricingBehavior != "" {
		item.PricingBehavior = input.PricingBehavior
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SaveBillableItem(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteBillableItem(id string) (*models.BillableItem, error) {
	item, err := s.repo.FindBillableItemByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteBillableItem(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreatePackage(input CreatePackageInput) (*models.CommercialPackage, error) {
	item := &models.CommercialPackage{
		ID:          utils.GenerateID(),
		ProductID:   input.ProductID,
		Code:        input.Code,
		Name:        input.Name,
		PackageType: input.PackageType,
		Status:      defaultString(input.Status, "active"),
		Metadata:    input.Metadata,
	}
	if err := s.repo.CreatePackage(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListPackages(productID string) ([]models.CommercialPackage, error) {
	return s.repo.ListPackages(productID)
}

func (s *Service) GetPackage(id string) (*models.CommercialPackage, error) {
	return s.repo.FindPackageByID(id)
}

func (s *Service) UpdatePackage(id string, input UpdatePackageInput) (*models.CommercialPackage, error) {
	item, err := s.repo.FindPackageByID(id)
	if err != nil {
		return nil, err
	}
	if input.Name != "" {
		item.Name = input.Name
	}
	if input.PackageType != "" {
		item.PackageType = input.PackageType
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SavePackage(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeletePackage(id string) (*models.CommercialPackage, error) {
	item, err := s.repo.FindPackageByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeletePackage(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) CreateRateCard(input CreateRateCardInput) (*models.RateCard, error) {
	effectiveFrom, effectiveTo, err := parseEffectiveWindow(input.EffectiveFrom, input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	item := &models.RateCard{
		ID:            utils.GenerateID(),
		ProductID:     input.ProductID,
		Code:          input.Code,
		TargetType:    input.TargetType,
		TargetID:      input.TargetID,
		PriceModel:    input.PriceModel,
		Currency:      defaultString(input.Currency, "CNY"),
		PriceConfig:   input.PriceConfig,
		EffectiveFrom: effectiveFrom,
		EffectiveTo:   effectiveTo,
		Version:       defaultInt(input.Version, 1),
		Status:        defaultString(input.Status, "active"),
		Metadata:      input.Metadata,
	}
	if err := s.repo.CreateRateCard(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) ListRateCards(productID, targetType string) ([]models.RateCard, error) {
	return s.repo.ListRateCards(productID, targetType)
}

func (s *Service) GetRateCard(id string) (*models.RateCard, error) {
	return s.repo.FindRateCardByID(id)
}

func (s *Service) UpdateRateCard(id string, input UpdateRateCardInput) (*models.RateCard, error) {
	item, err := s.repo.FindRateCardByID(id)
	if err != nil {
		return nil, err
	}
	effectiveFrom, effectiveTo, err := parseEffectiveWindow(input.EffectiveFrom, input.EffectiveTo)
	if err != nil {
		return nil, err
	}
	if input.PriceModel != "" {
		item.PriceModel = input.PriceModel
	}
	if input.Currency != "" {
		item.Currency = input.Currency
	}
	if input.PriceConfig != "" {
		item.PriceConfig = input.PriceConfig
	}
	if input.EffectiveFrom != "" || input.EffectiveTo != "" {
		item.EffectiveFrom = effectiveFrom
		item.EffectiveTo = effectiveTo
	}
	if input.Version > 0 {
		item.Version = input.Version
	}
	if input.Status != "" {
		item.Status = input.Status
	}
	if input.Metadata != "" {
		item.Metadata = input.Metadata
	}
	if err := s.repo.SaveRateCard(item); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) DeleteRateCard(id string) (*models.RateCard, error) {
	item, err := s.repo.FindRateCardByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.repo.DeleteRateCard(id); err != nil {
		return nil, err
	}
	return item, nil
}

func (s *Service) Offerings(productCode string, financeRepo *repository.FinanceRepository) (*OfferingsView, error) {
	product, err := s.repo.FindProductByCode(productCode)
	if err != nil {
		return nil, err
	}
	skus, err := s.repo.ListSKUs(product.ID)
	if err != nil {
		return nil, err
	}
	packages, err := s.repo.ListPackages(product.ID)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.ListBillableItems(product.ID)
	if err != nil {
		return nil, err
	}
	cards, err := s.repo.ListRateCards(product.ID, "")
	if err != nil {
		return nil, err
	}
	assetDefinitions, err := financeRepo.ListAssetDefinitions(productCode, "", "")
	if err != nil {
		return nil, err
	}
	allowancePolicies, err := financeRepo.ListAllowancePolicies(productCode, "", "")
	if err != nil {
		return nil, err
	}
	return &OfferingsView{
		Product:           product,
		SKUs:              skus,
		Packages:          packages,
		BillableItems:     items,
		RateCards:         cards,
		AssetDefinitions:  assetDefinitions,
		AllowancePolicies: allowancePolicies,
	}, nil
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func defaultInt(value, fallback int) int {
	if value == 0 {
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
