package runtime

import (
	"encoding/json"
	"slices"
	"sort"
	"strings"

	"platform-service/internal/models"
)

type RuntimeRouteSnapshot struct {
	Objective          string   `json:"objective,omitempty"`
	PreferredProviders []string `json:"preferred_providers,omitempty"`
	CandidateProviders []string `json:"candidate_providers,omitempty"`
	CurrentProviderIdx int      `json:"current_provider_idx,omitempty"`
}

type bindingMetadata struct {
	ObjectiveScores map[string]int `json:"objective_scores,omitempty"`
	FallbackOn      []string       `json:"fallback_on,omitempty"`
}

func decodeRouteSnapshot(raw string) RuntimeRouteSnapshot {
	if strings.TrimSpace(raw) == "" {
		return RuntimeRouteSnapshot{Objective: "balanced"}
	}
	var out RuntimeRouteSnapshot
	_ = json.Unmarshal([]byte(raw), &out)
	if strings.TrimSpace(out.Objective) == "" {
		out.Objective = "balanced"
	}
	return out
}

func encodeRouteSnapshot(snapshot RuntimeRouteSnapshot) string {
	body, _ := json.Marshal(snapshot)
	return string(body)
}

func decodeBindingMetadata(raw string) bindingMetadata {
	if strings.TrimSpace(raw) == "" {
		return bindingMetadata{}
	}
	var out bindingMetadata
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

func rankProviderBindings(bindings []models.RuntimeProviderBinding, snapshot RuntimeRouteSnapshot) []models.RuntimeProviderBinding {
	if len(bindings) == 0 {
		return nil
	}
	preferredOrder := map[string]int{}
	for idx, providerCode := range snapshot.PreferredProviders {
		preferredOrder[strings.TrimSpace(providerCode)] = idx
	}
	sort.SliceStable(bindings, func(i, j int) bool {
		left := bindings[i]
		right := bindings[j]
		leftPreferredIdx, leftPreferred := preferredOrder[left.ProviderCode]
		rightPreferredIdx, rightPreferred := preferredOrder[right.ProviderCode]
		switch {
		case leftPreferred && rightPreferred:
			return leftPreferredIdx < rightPreferredIdx
		case leftPreferred:
			return true
		case rightPreferred:
			return false
		}

		leftScore := objectiveScore(left, snapshot.Objective)
		rightScore := objectiveScore(right, snapshot.Objective)
		if leftScore != rightScore {
			return leftScore > rightScore
		}
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return left.ProviderCode < right.ProviderCode
	})
	return bindings
}

func objectiveScore(binding models.RuntimeProviderBinding, objective string) int {
	switch strings.ToLower(strings.TrimSpace(objective)) {
	case "", "balanced":
		return 0
	}
	metadata := decodeBindingMetadata(binding.Metadata)
	if metadata.ObjectiveScores == nil {
		return 0
	}
	return metadata.ObjectiveScores[strings.ToLower(strings.TrimSpace(objective))]
}

func candidateProviderCodes(bindings []models.RuntimeProviderBinding) []string {
	out := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.ProviderCode == "" || slices.Contains(out, binding.ProviderCode) {
			continue
		}
		out = append(out, binding.ProviderCode)
	}
	return out
}

func fallbackAllowed(binding *models.RuntimeProviderBinding, errorClass string) bool {
	if binding == nil {
		return false
	}
	metadata := decodeBindingMetadata(binding.Metadata)
	if len(metadata.FallbackOn) == 0 {
		return errorClass == "retryable_provider" || errorClass == "provider_unavailable" || errorClass == "provider_timeout"
	}
	return slices.Contains(metadata.FallbackOn, errorClass)
}
