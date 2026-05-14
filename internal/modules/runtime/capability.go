package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"platform-service/internal/models"
	"platform-service/pkg/platformconst"

	"gorm.io/gorm"
)

const (
	RuntimeTaskImageUnderstanding = "image_understanding"
	RuntimeTaskOCR                = "ocr"
	RuntimeTaskImageGeneration    = "image_generation"
	RuntimeTaskImageInpainting    = "image_inpainting"
	RuntimeTaskVideoKeyframe      = "video_keyframe"
	RuntimeTaskTextReasoning      = "text_reasoning"
	RuntimeTaskIntentPlanning     = "intent_planning"
	RuntimeTaskPromptPlanning     = "prompt_planning"
	RuntimeTaskStrategyReport     = "strategy_report"

	RuntimeCapabilityStatusAvailable   = "available"
	RuntimeCapabilityStatusUnavailable = "unavailable"
	RuntimeContractStatusReady         = "ready"
	RuntimeContractStatusNeeded        = "contract-needed"

	RuntimeCapabilityReasonContractNeeded          = "contract-needed"
	RuntimeCapabilityReasonProviderBindingMissing  = "provider_binding_missing"
	RuntimeCapabilityReasonProviderBindingDisabled = "provider_binding_disabled"
	RuntimeCapabilityReasonProviderNotRegistered   = "provider_not_registered"
	RuntimeCapabilityReasonProviderInactive        = "provider_inactive"
	RuntimeCapabilityReasonCallbackEndpointMissing = "callback_endpoint_missing"
	RuntimeCapabilityReasonCallbackKindUnsupported = "callback_kind_unsupported"
	RuntimeCapabilityReasonStorageBindingMissing   = "storage_binding_missing"
	RuntimeCapabilityReasonBillableItemMissing     = "billable_item_missing"
)

var errRuntimeTaskTypeUnknown = errors.New("unknown runtime task_type")

type runtimeTaskContractDefinition struct {
	TaskType         string
	ContractStatus   string
	BillableItemCode string
}

var runtimeTaskContractDefinitions = []runtimeTaskContractDefinition{
	{TaskType: RuntimeTaskImageUnderstanding, ContractStatus: RuntimeContractStatusNeeded},
	{TaskType: RuntimeTaskOCR, ContractStatus: RuntimeContractStatusNeeded},
	{TaskType: RuntimeTaskImageGeneration, ContractStatus: RuntimeContractStatusReady},
	{TaskType: RuntimeTaskImageInpainting, ContractStatus: RuntimeContractStatusNeeded},
	{TaskType: RuntimeTaskVideoKeyframe, ContractStatus: RuntimeContractStatusNeeded},
	{TaskType: RuntimeTaskTextReasoning, ContractStatus: RuntimeContractStatusReady},
	{TaskType: RuntimeTaskIntentPlanning, ContractStatus: RuntimeContractStatusReady},
	{TaskType: RuntimeTaskPromptPlanning, ContractStatus: RuntimeContractStatusReady},
	{TaskType: RuntimeTaskStrategyReport, ContractStatus: RuntimeContractStatusNeeded},
}

type RuntimeCapabilityMatrix struct {
	ProductCode string                  `json:"product_code"`
	Items       []RuntimeCapabilityItem `json:"items"`
}

type RuntimeCapabilityItem struct {
	TaskType          string                             `json:"task_type"`
	Status            string                             `json:"status"`
	Available         bool                               `json:"available"`
	UnavailableReason string                             `json:"unavailable_reason"`
	ContractStatus    string                             `json:"contract_status"`
	ProviderBindings  []RuntimeProviderBindingCapability `json:"provider_bindings"`
	Callback          RuntimeCallbackCapability          `json:"callback"`
	Storage           RuntimeStorageCapability           `json:"storage"`
	Billing           RuntimeBillingCapability           `json:"billing"`
	Reasons           []RuntimeCapabilityReason          `json:"reasons"`
}

type RuntimeProviderBindingCapability struct {
	ProviderCode string         `json:"provider_code"`
	Enabled      bool           `json:"enabled"`
	Registered   bool           `json:"registered"`
	Status       string         `json:"status"`
	Priority     int            `json:"priority"`
	Model        string         `json:"model"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

type RuntimeCallbackCapability struct {
	Configured   bool   `json:"configured"`
	CallbackKind string `json:"callback_kind"`
}

type RuntimeStorageCapability struct {
	OutputCategory    string `json:"output_category"`
	BindingConfigured bool   `json:"binding_configured"`
}

type RuntimeBillingCapability struct {
	BillableItemCode string `json:"billable_item_code"`
	MeterUnit        string `json:"meter_unit"`
	SettlementMode   string `json:"settlement_mode"`
	Configured       bool   `json:"configured"`
}

type RuntimeCapabilityReason struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Service) ListRuntimeCapabilities(productCode, taskType string) (*RuntimeCapabilityMatrix, error) {
	productCode = strings.TrimSpace(productCode)
	taskType = strings.TrimSpace(taskType)
	if productCode == "" {
		return nil, fmt.Errorf("product_code is required")
	}

	definitions := runtimeTaskContractDefinitions
	if taskType != "" {
		definition, ok := findRuntimeTaskDefinition(taskType)
		if !ok {
			return nil, fmt.Errorf("%w: %s", errRuntimeTaskTypeUnknown, taskType)
		}
		definitions = []runtimeTaskContractDefinition{definition}
	}

	endpoint, endpointErr := s.repo.FindActiveProductEndpoint(productCode)
	if endpointErr != nil && !errors.Is(endpointErr, gorm.ErrRecordNotFound) {
		return nil, endpointErr
	}
	var endpointValue *models.RuntimeProductEndpoint
	if endpointErr == nil {
		endpointValue = endpoint
	}
	outputCategory := resolveOutputStorageCategory(endpointValue, productCode, s.repo.ListStorageBindings)
	storageBinding, storageErr := s.repo.FindPreferredStorageBinding(productCode, outputCategory)
	storageConfigured := storageErr == nil && storageBinding != nil && storageBinding.Enabled
	if storageErr != nil && !errors.Is(storageErr, gorm.ErrRecordNotFound) {
		return nil, storageErr
	}

	items := make([]RuntimeCapabilityItem, 0, len(definitions))
	for _, definition := range definitions {
		item, err := s.buildRuntimeCapabilityItem(productCode, definition, endpointValue, outputCategory, storageConfigured)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &RuntimeCapabilityMatrix{ProductCode: productCode, Items: items}, nil
}

func (s *Service) buildRuntimeCapabilityItem(productCode string, definition runtimeTaskContractDefinition, endpoint *models.RuntimeProductEndpoint, outputCategory string, storageConfigured bool) (RuntimeCapabilityItem, error) {
	billableItemCode := runtimeBillableItemCode(productCode, definition)
	item := RuntimeCapabilityItem{
		TaskType:       definition.TaskType,
		Status:         RuntimeCapabilityStatusUnavailable,
		ContractStatus: definition.ContractStatus,
		Callback: RuntimeCallbackCapability{
			Configured: endpoint != nil,
		},
		Storage: RuntimeStorageCapability{
			OutputCategory:    outputCategory,
			BindingConfigured: storageConfigured,
		},
		Billing: RuntimeBillingCapability{
			BillableItemCode: billableItemCode,
		},
	}
	if endpoint != nil {
		item.Callback.CallbackKind = endpoint.CallbackKind
	}

	if definition.ContractStatus != RuntimeContractStatusReady {
		item.addReason(RuntimeCapabilityReasonContractNeeded)
	}
	if endpoint == nil {
		item.addReason(RuntimeCapabilityReasonCallbackEndpointMissing)
	} else if buildProductCallbackClient(endpoint) == nil {
		item.addReason(RuntimeCapabilityReasonCallbackKindUnsupported)
	}
	if !storageConfigured {
		item.addReason(RuntimeCapabilityReasonStorageBindingMissing)
	}

	billableItem, billableErr := s.findActiveRuntimeBillableItem(productCode, billableItemCode)
	if billableErr != nil {
		return item, billableErr
	}
	if billableItem == nil {
		item.addReason(RuntimeCapabilityReasonBillableItemMissing)
	} else {
		item.Billing.MeterUnit = billableItem.MeterUnit
		item.Billing.SettlementMode = billableItem.SettlementMode
		item.Billing.Configured = true
	}

	providerBindings, err := s.repo.ListAllProviderBindings(productCode, definition.TaskType)
	if err != nil {
		return item, err
	}
	if len(providerBindings) == 0 {
		item.addReason(RuntimeCapabilityReasonProviderBindingMissing)
	} else {
		usableProvider := false
		hasEnabled := false
		item.ProviderBindings = make([]RuntimeProviderBindingCapability, 0, len(providerBindings))
		for _, binding := range providerBindings {
			if binding.Enabled {
				hasEnabled = true
			}
			capability := RuntimeProviderBindingCapability{
				ProviderCode: binding.ProviderCode,
				Enabled:      binding.Enabled,
				Priority:     binding.Priority,
				Model:        binding.Model,
				Metadata:     parseRuntimeCapabilityMetadata(binding.Metadata),
			}
			providerDefinition, defErr := s.repo.FindProviderDefinitionByCode(binding.ProviderCode)
			if defErr != nil && !errors.Is(defErr, gorm.ErrRecordNotFound) {
				return item, defErr
			}
			if providerDefinition != nil {
				capability.Status = providerDefinition.Status
			} else if capability.Status == "" {
				capability.Status = "unknown"
			}
			if s.registry != nil {
				_, registryErr := s.registry.Get(binding.ProviderCode)
				capability.Registered = registryErr == nil
			} else {
				capability.Registered = binding.ProviderCode == "manual" || binding.ProviderCode == "mock" || binding.ProviderCode == "volcengine"
			}
			providerActive := providerDefinition == nil || providerDefinition.Status == platformconst.StatusActive
			if binding.Enabled && capability.Registered && providerActive {
				usableProvider = true
			}
			if binding.Enabled && !capability.Registered {
				item.addReason(RuntimeCapabilityReasonProviderNotRegistered)
			}
			if binding.Enabled && !providerActive {
				item.addReason(RuntimeCapabilityReasonProviderInactive)
			}
			item.ProviderBindings = append(item.ProviderBindings, capability)
		}
		if !hasEnabled {
			item.addReason(RuntimeCapabilityReasonProviderBindingDisabled)
		}
		if hasEnabled && !usableProvider && !item.hasReason(RuntimeCapabilityReasonProviderNotRegistered) && !item.hasReason(RuntimeCapabilityReasonProviderInactive) {
			item.addReason(RuntimeCapabilityReasonProviderNotRegistered)
		}
	}

	item.Available = len(item.Reasons) == 0
	if item.Available {
		item.Status = RuntimeCapabilityStatusAvailable
	} else {
		item.UnavailableReason = item.Reasons[0].Code
	}
	return item, nil
}

func (s *Service) findActiveRuntimeBillableItem(productCode, billableItemCode string) (*models.BillableItem, error) {
	var product models.Product
	if err := s.repo.DB().Where("code = ? AND status = ?", productCode, platformconst.StatusActive).First(&product).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	var item models.BillableItem
	if err := s.repo.DB().Where("product_id = ? AND code = ? AND status = ?", product.ID, billableItemCode, platformconst.StatusActive).First(&item).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &item, nil
}

func findRuntimeTaskDefinition(taskType string) (runtimeTaskContractDefinition, bool) {
	for _, definition := range runtimeTaskContractDefinitions {
		if definition.TaskType == taskType {
			return definition, true
		}
	}
	return runtimeTaskContractDefinition{}, false
}

func resolveOutputStorageCategory(endpoint *models.RuntimeProductEndpoint, productCode string, listStorageBindings func(string) ([]models.StorageBinding, error)) string {
	if endpoint != nil && endpoint.Metadata != "" {
		var metadata map[string]any
		if err := json.Unmarshal([]byte(endpoint.Metadata), &metadata); err == nil {
			if category, ok := metadata["output_storage_category"].(string); ok && strings.TrimSpace(category) != "" {
				return strings.TrimSpace(category)
			}
		}
	}
	if bindings, err := listStorageBindings(productCode); err == nil {
		for _, binding := range bindings {
			if binding.Category == "runtime-assets" {
				return binding.Category
			}
		}
		for _, binding := range bindings {
			if binding.Category != "" && binding.Category != "*" {
				return binding.Category
			}
		}
	}
	return "runtime-assets"
}

func parseRuntimeCapabilityMetadata(raw string) map[string]any {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil
	}
	return sanitizeRuntimeCapabilityMetadata(out)
}

func runtimeBillableItemCode(productCode string, definition runtimeTaskContractDefinition) string {
	if strings.TrimSpace(definition.BillableItemCode) != "" {
		return strings.TrimSpace(definition.BillableItemCode)
	}
	return fmt.Sprintf("%s_runtime_%s", strings.TrimSpace(productCode), strings.TrimSpace(definition.TaskType))
}

func sanitizeRuntimeCapabilityMetadata(raw map[string]any) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	allowed := map[string]bool{
		"display_name":             true,
		"description":              true,
		"model_family":             true,
		"quality_tier":             true,
		"input_modes":              true,
		"output_mime_types":        true,
		"max_variants":             true,
		"supports_inpainting":      true,
		"supports_reference_image": true,
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		cleanKey := strings.TrimSpace(key)
		if allowed[cleanKey] {
			out[cleanKey] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (item *RuntimeCapabilityItem) addReason(code string) {
	if item.hasReason(code) {
		return
	}
	item.Reasons = append(item.Reasons, RuntimeCapabilityReason{Code: code, Message: runtimeCapabilityReasonMessage(code)})
}

func (item RuntimeCapabilityItem) hasReason(code string) bool {
	for _, reason := range item.Reasons {
		if reason.Code == code {
			return true
		}
	}
	return false
}

func runtimeCapabilityReasonMessage(code string) string {
	switch code {
	case RuntimeCapabilityReasonContractNeeded:
		return "Runtime task contract is not implemented for this task_type yet."
	case RuntimeCapabilityReasonProviderBindingMissing:
		return "No provider binding exists for product_code + task_type."
	case RuntimeCapabilityReasonProviderBindingDisabled:
		return "Provider bindings exist but all are disabled for product_code + task_type."
	case RuntimeCapabilityReasonProviderNotRegistered:
		return "Provider binding references a provider not registered by the runtime registry."
	case RuntimeCapabilityReasonProviderInactive:
		return "Provider definition exists but is not active."
	case RuntimeCapabilityReasonCallbackEndpointMissing:
		return "No active runtime product callback endpoint is configured for product_code."
	case RuntimeCapabilityReasonCallbackKindUnsupported:
		return "Runtime product callback endpoint uses an unsupported callback_kind."
	case RuntimeCapabilityReasonStorageBindingMissing:
		return "No usable storage binding is configured for the resolved output category."
	case RuntimeCapabilityReasonBillableItemMissing:
		return "No active billable item is configured for the expected runtime task."
	default:
		return "Runtime capability is unavailable."
	}
}
