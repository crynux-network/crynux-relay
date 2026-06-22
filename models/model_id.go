package models

import "strings"

func NormalizeModelID(modelID string) string {
	return strings.ToLower(modelID)
}

func NormalizeModelIDs(modelIDs []string) []string {
	normalized := make([]string, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		normalized = append(normalized, NormalizeModelID(modelID))
	}
	return normalized
}
