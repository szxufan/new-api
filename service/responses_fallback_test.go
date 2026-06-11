package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldFallbackResponsesToChat_Default(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Should return false by default (no fallback)
	result := ShouldFallbackResponsesToChat(123, "gpt-4o")
	assert.False(t, result)
}

func TestMarkResponsesFallback(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Mark as needing fallback
	MarkResponsesFallback(123, "gpt-4o")

	// Should now return true
	result := ShouldFallbackResponsesToChat(123, "gpt-4o")
	assert.True(t, result)
}

func TestShouldFallbackResponsesToChat_DifferentChannel(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Mark channel 123 as needing fallback
	MarkResponsesFallback(123, "gpt-4o")

	// Channel 456 should not be affected
	result := ShouldFallbackResponsesToChat(456, "gpt-4o")
	assert.False(t, result)
}

func TestShouldFallbackResponsesToChat_DifferentModel(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Mark channel 123, model gpt-4o as needing fallback
	MarkResponsesFallback(123, "gpt-4o")

	// Same channel but different model should not be affected
	result := ShouldFallbackResponsesToChat(123, "gpt-4o-mini")
	assert.False(t, result)
}

func TestClearResponsesFallbackForChannel(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Mark as needing fallback
	MarkResponsesFallback(123, "gpt-4o")
	assert.True(t, ShouldFallbackResponsesToChat(123, "gpt-4o"))

	// Clear for specific channel+model
	ClearResponsesFallbackForChannel(123, "gpt-4o")

	// Should now return false
	result := ShouldFallbackResponsesToChat(123, "gpt-4o")
	assert.False(t, result)
}

func TestClearResponsesFallbackCache(t *testing.T) {
	// Mark multiple entries
	MarkResponsesFallback(123, "gpt-4o")
	MarkResponsesFallback(456, "gpt-4o-mini")
	MarkResponsesFallback(789, "claude-3")

	// Verify they exist
	assert.True(t, ShouldFallbackResponsesToChat(123, "gpt-4o"))
	assert.True(t, ShouldFallbackResponsesToChat(456, "gpt-4o-mini"))
	assert.True(t, ShouldFallbackResponsesToChat(789, "claude-3"))

	// Clear all
	ClearResponsesFallbackCache()

	// All should return false
	assert.False(t, ShouldFallbackResponsesToChat(123, "gpt-4o"))
	assert.False(t, ShouldFallbackResponsesToChat(456, "gpt-4o-mini"))
	assert.False(t, ShouldFallbackResponsesToChat(789, "claude-3"))
}

func TestGetResponsesFallbackCacheSize(t *testing.T) {
	// Clear cache before test
	ClearResponsesFallbackCache()

	// Initial size should be 0
	assert.Equal(t, 0, GetResponsesFallbackCacheSize())

	// Add entries
	MarkResponsesFallback(123, "gpt-4o")
	assert.Equal(t, 1, GetResponsesFallbackCacheSize())

	MarkResponsesFallback(456, "gpt-4o-mini")
	assert.Equal(t, 2, GetResponsesFallbackCacheSize())

	// Same key should not increase size
	MarkResponsesFallback(123, "gpt-4o")
	assert.Equal(t, 2, GetResponsesFallbackCacheSize())

	// Clear
	ClearResponsesFallbackCache()
	assert.Equal(t, 0, GetResponsesFallbackCacheSize())
}

func TestMakeFallbackKey(t *testing.T) {
	key := makeFallbackKey(123, "gpt-4o")
	assert.Equal(t, "123:gpt-4o", key)

	key2 := makeFallbackKey(456, "claude-3-opus")
	assert.Equal(t, "456:claude-3-opus", key2)
}