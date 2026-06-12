package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"platform-service/internal/models"
	assetstorage "platform-service/internal/modules/assetstorage"
	"platform-service/pkg/platformconst"
)

func (s *Service) completeRuntimeJob(job *models.RuntimeJob, _ RuntimeInputManifest, completion *ProviderCompletion, now time.Time) error {
	variants := make([]ProductRecordResultVariant, 0, len(completion.Variants))
	manifestVariants := make([]RuntimeOutputVariantManifest, 0, len(completion.Variants))
	outputCategory := s.outputStorageCategory(job)
	for _, variant := range completion.Variants {
		sourceURL := variant.SourceURL
		previewURL := firstNonEmpty(variant.PreviewURL, variant.SourceURL)
		storageKey := ""
		storageAssetID := ""
		var fileSize int64
		if s.storage != nil {
			switch {
			case strings.TrimSpace(variant.InlineData) != "":
				stored, storeErr := s.storage.UploadAsset(context.Background(), assetstorage.UploadAssetInput{
					ProductCode: job.ProductCode,
					Category:    outputCategory,
					FileName:    "",
					MimeType:    variant.MimeType,
					Payload:     runtimeInlineStoragePayload(variant.InlineData, variant.MimeType),
				})
				if storeErr != nil {
					s.runtimeJobLogger(job).
						With("variant_index", variant.Index, "output_storage_category", outputCategory, "storage_operation", "upload_inline", "error", storeErr).
						Error("runtime.result.store_failed")
					return storeErr
				}
				storageKey = stored.StorageKey
				fileSize = stored.FileSize
				sourceURL = ""
				previewURL = ""
				variant.MimeType = firstNonEmpty(stored.MimeType, variant.MimeType)
			case strings.TrimSpace(sourceURL) != "":
				stored, storeErr := s.storage.ImportRemoteAsset(context.Background(), job.ProductCode, outputCategory, "", variant.MimeType, sourceURL)
				if storeErr != nil {
					s.runtimeJobLogger(job).
						With("variant_index", variant.Index, "output_storage_category", outputCategory, "storage_operation", "import_remote", "error", storeErr).
						Error("runtime.result.store_failed")
					return storeErr
				}
				storageKey = stored.StorageKey
				fileSize = stored.FileSize
				// Preserve the provider/result URL in the runtime output contract even
				// after importing a copy into Platform storage. Product frontends need a
				// directly playable URL while storage_key remains the auditable archive.
				variant.MimeType = firstNonEmpty(stored.MimeType, variant.MimeType)
			}
		}
		assetType := firstNonEmpty(variant.AssetType, runtimeAssetTypeForTask(job.TaskType))
		assetMetadata := sanitizeProviderCallbackMetadata(variant.Metadata)
		callbackInlineData := ""
		callbackText := ""
		mimeLower := strings.ToLower(strings.TrimSpace(variant.MimeType))
		if (strings.HasPrefix(mimeLower, "text/") || strings.Contains(mimeLower, "json")) && strings.TrimSpace(variant.InlineData) != "" {
			callbackInlineData = variant.InlineData
			callbackText = variant.InlineData
		}
		if s.storage != nil && strings.TrimSpace(storageKey) != "" {
			registered, registerErr := s.storage.RegisterAsset(context.Background(), assetstorage.RegisterAssetInput{
				ProductCode: job.ProductCode,
				Category:    outputCategory,
				SourceType:  "runtime_output",
				SourceRef:   fmt.Sprintf("%s:%d", job.ID, variant.Index),
				StorageKey:  storageKey,
				MimeType:    variant.MimeType,
				FileSize:    fileSize,
				Metadata: map[string]any{
					"runtime_job_id": job.ID,
					"task_type":      job.TaskType,
					"variant_index":  variant.Index,
					"provider":       job.ProviderCode,
				},
				Status: "active",
			})
			if registerErr != nil {
				s.runtimeJobLogger(job).
					With("variant_index", variant.Index, "output_storage_category", outputCategory, "storage_operation", "register_output", "error", registerErr).
					Error("runtime.result.register_failed")
				return registerErr
			}
			if registered != nil {
				storageAssetID = registered.ID
				if fileSize == 0 {
					fileSize = registered.FileSize
				}
			}
		}
		variants = append(variants, ProductRecordResultVariant{
			Index:      variant.Index,
			Status:     "ready",
			IsSelected: variant.Index == 0,
			InlineData: callbackInlineData,
			Text:       callbackText,
			Asset: ProductRecordResultAsset{
				AssetType:      assetType,
				SourceType:     "runtime_output",
				FileName:       "",
				StorageKey:     storageKey,
				StorageAssetID: storageAssetID,
				SourceURL:      sourceURL,
				PreviewURL:     previewURL,
				MimeType:       variant.MimeType,
				FileSize:       fileSize,
				Width:          variant.Width,
				Height:         variant.Height,
				Metadata:       assetMetadata,
			},
		})
		manifestVariants = append(manifestVariants, RuntimeOutputVariantManifest{
			Index:      variant.Index,
			Status:     "ready",
			IsSelected: variant.Index == 0,
			Asset: RuntimeOutputAssetManifest{
				AssetType:      assetType,
				SourceType:     "runtime_output",
				StorageKey:     storageKey,
				StorageAssetID: storageAssetID,
				SourceURL:      sourceURL,
				PreviewURL:     previewURL,
				MimeType:       variant.MimeType,
				FileSize:       fileSize,
				Width:          variant.Width,
				Height:         variant.Height,
				Metadata:       assetMetadata,
			},
		})
	}
	outputManifest := mustMarshal(RuntimeOutputManifest{
		Contract:     "platform.runtime.output.v1",
		RuntimeJobID: job.ID,
		ProductCode:  job.ProductCode,
		TaskType:     job.TaskType,
		ProviderCode: job.ProviderCode,
		Status:       platformconst.StatusCompleted,
		Progress:     completion.Progress,
		StageMessage: completion.StageMessage,
		Storage: map[string]any{
			"output_category": outputCategory,
			"registry":        "platform_storage_assets",
		},
		Variants:     manifestVariants,
		ProviderMeta: sanitizeProviderCallbackMetadata(completion.Metadata),
	})
	if _, _, err := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
		Event:          RuntimeJobEventCompleted,
		Now:            now,
		Stage:          platformconst.StatusCompleted,
		StageMessage:   completion.StageMessage,
		OutputManifest: outputManifest,
	}); err != nil {
		return err
	}
	s.runtimeJobLogger(job).
		With("variant_count", len(variants), "output_storage_category", outputCategory).
		Info("runtime.completed")
	resultMetadata := sanitizeProviderCallbackMetadata(completion.Metadata)
	if resultMetadata == nil {
		resultMetadata = map[string]any{}
	}
	resultMetadata["source_type"] = job.SourceType
	resultMetadata["runtime_job_id"] = job.ID
	if err := s.notifyProductResults(job, ProductRecordResultsInput{
		Status:       platformconst.StatusCompleted,
		Progress:     completion.Progress,
		StageMessage: completion.StageMessage,
		Metadata:     resultMetadata,
		Variants:     variants,
	}); err != nil {
		s.runtimeJobLogger(job).
			With("variant_count", len(variants), "output_storage_category", outputCategory, "error", err).
			Warn("runtime.results.notify_failed")
		if _, _, saveErr := s.transitionRuntimeJob(job, RuntimeJobTransitionInput{
			Event:        RuntimeJobEventTerminalMetadataPatch,
			Now:          time.Now(),
			Stage:        "callback_results_failed",
			StageMessage: "Result callback failed; runtime output remains available",
			ErrorClass:   "callback_failed",
			ErrorCode:    "PRODUCT_RESULT_CALLBACK_FAILED",
			ErrorMessage: err.Error(),
		}); saveErr != nil {
			return saveErr
		}
		return nil
	}
	return nil
}

func runtimeAssetTypeForTask(taskType string) string {
	switch taskType {
	case RuntimeTaskImageUnderstanding:
		return "json"
	case RuntimeTaskTextReasoning, RuntimeTaskIntentPlanning, RuntimeTaskPromptPlanning, RuntimeTaskStrategyReport:
		return "text"
	default:
		return "generated"
	}
}

func (s *Service) outputStorageCategory(job *models.RuntimeJob) string {
	if job == nil {
		return "runtime-assets"
	}
	if endpoint := s.productEndpoint(job.ProductCode); endpoint != nil {
		metadata := decodeJSONMap(endpoint.Metadata)
		if category, ok := metadata["output_storage_category"].(string); ok && strings.TrimSpace(category) != "" {
			return category
		}
	}
	if binding, err := s.repo.FindPreferredStorageBinding(job.ProductCode, "runtime-assets"); err == nil && binding != nil && strings.TrimSpace(binding.Category) != "" {
		return binding.Category
	}
	if binding, err := s.repo.FindPreferredStorageBinding(job.ProductCode, "*"); err == nil && binding != nil && strings.TrimSpace(binding.Category) != "" && binding.Category != "*" {
		return binding.Category
	}
	if bindings, err := s.repo.ListStorageBindings(job.ProductCode); err == nil {
		for _, binding := range bindings {
			category := strings.TrimSpace(binding.Category)
			if category == "" || category == "*" {
				continue
			}
			return category
		}
	}
	return "runtime-assets"
}
