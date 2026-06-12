package service

import (
	"fmt"
	"sync"
)

// responsesFallbackCache stores whether a channel+model combination should fallback
// from /v1/responses to /v1/chat/completions.
// Key format: "channelID:modelName"
// Value: true (should fallback) or false (not marked)
var responsesFallbackCache sync.Map

// ShouldFallbackResponsesToChat checks if the channel+model combination should
// use /v1/chat/completions instead of /v1/responses.
// Returns true if the endpoint has been marked as unsupported (received 404).
func ShouldFallbackResponsesToChat(channelID int, modelName string) bool {
	key := makeFallbackKey(channelID, modelName)
	val, ok := responsesFallbackCache.Load(key)
	if ok {
		return val.(bool)
	}
	return false // Default: try /v1/responses first
}

// MarkResponsesFallback marks a channel+model combination as needing fallback.
// Called when a 404 response is received from /v1/responses endpoint.
func MarkResponsesFallback(channelID int, modelName string) {
	key := makeFallbackKey(channelID, modelName)
	responsesFallbackCache.Store(key, true)
}

// ClearResponsesFallbackCache clears all cached fallback decisions.
// Useful for testing or manual reset.
func ClearResponsesFallbackCache() {
	responsesFallbackCache = sync.Map{}
}

// ClearResponsesFallbackForChannel clears fallback decisions for a specific channel+model.
func ClearResponsesFallbackForChannel(channelID int, modelName string) {
	key := makeFallbackKey(channelID, modelName)
	responsesFallbackCache.Delete(key)
}

// GetResponsesFallbackCacheSize returns the number of entries in the cache.
// Useful for monitoring/debugging.
func GetResponsesFallbackCacheSize() int {
	count := 0
	responsesFallbackCache.Range(func(key, value any) bool {
		count++
		return true
	})
	return count
}

// makeFallbackKey creates the cache key from channelID and modelName.
func makeFallbackKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, modelName)
}

// --- Strip ResponseFormat cache ---
// Stores whether a channel+model combination cannot handle both response_format and tools.
// Key format: "channelID:modelName"
// Value: true (should strip response_format)

var stripResponseFormatCache sync.Map

// ShouldStripResponseFormat checks if the channel+model combination should have
// response_format stripped (known to be incompatible with tools/functions).
func ShouldStripResponseFormat(channelID int, modelName string) bool {
	key := makeStripRFKey(channelID, modelName)
	val, ok := stripResponseFormatCache.Load(key)
	if ok {
		return val.(bool)
	}
	return false
}

// MarkStripResponseFormat marks a channel+model combination as needing
// response_format stripped when tools are present.
func MarkStripResponseFormat(channelID int, modelName string) {
	key := makeStripRFKey(channelID, modelName)
	stripResponseFormatCache.Store(key, true)
}

// ClearStripResponseFormatCache clears all cached strip decisions.
func ClearStripResponseFormatCache() {
	stripResponseFormatCache = sync.Map{}
}

// makeStripRFKey creates the cache key from channelID and modelName.
func makeStripRFKey(channelID int, modelName string) string {
	return fmt.Sprintf("%d:%s", channelID, modelName)
}