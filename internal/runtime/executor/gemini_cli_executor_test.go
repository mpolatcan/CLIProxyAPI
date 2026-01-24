package executor

import (
	"testing"
	"time"
)

func TestParseRetryDelay_UserQuotaError(t *testing.T) {
	// Error format from the user's 429 response
	userError := []byte(`{
		"error": {
			"code": 429,
			"message": "You have exhausted your capacity on this model. Your quota will reset after 12s.",
			"status": "RESOURCE_EXHAUSTED",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "RATE_LIMIT_EXCEEDED",
					"domain": "cloudcode-pa.googleapis.com",
					"metadata": {
						"uiMessage": "true",
						"model": "gemini-3-flash-preview"
					}
				}
			]
		}
	}`)

	delay, err := parseRetryDelay(userError)
	t.Logf("Parsed delay: %v, error: %v", delay, err)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if delay == nil {
		t.Fatal("Expected delay to be parsed, got nil")
	}
	expected := 12 * time.Second
	if *delay != expected {
		t.Errorf("Expected delay %v, got %v", expected, *delay)
	}
}

func TestParseRetryDelay_RetryInfo(t *testing.T) {
	// Error format with RetryInfo
	retryInfoError := []byte(`{
		"error": {
			"code": 429,
			"message": "Resource has been exhausted.",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.RetryInfo",
					"retryDelay": "0.847655010s"
				}
			]
		}
	}`)

	delay, err := parseRetryDelay(retryInfoError)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if delay == nil {
		t.Fatal("Expected delay to be parsed, got nil")
	}
	expected := 847655010 * time.Nanosecond
	if *delay != expected {
		t.Errorf("Expected delay %v, got %v", expected, *delay)
	}
}

func TestParseRetryDelay_QuotaResetDelay(t *testing.T) {
	// Error format with quotaResetDelay in ErrorInfo metadata
	quotaResetError := []byte(`{
		"error": {
			"code": 429,
			"message": "Resource has been exhausted.",
			"details": [
				{
					"@type": "type.googleapis.com/google.rpc.ErrorInfo",
					"reason": "RATE_LIMIT_EXCEEDED",
					"domain": "googleapis.com",
					"metadata": {
						"quotaResetDelay": "373.801628ms"
					}
				}
			]
		}
	}`)

	delay, err := parseRetryDelay(quotaResetError)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if delay == nil {
		t.Fatal("Expected delay to be parsed, got nil")
	}
	expected := 373801628 * time.Nanosecond
	if *delay != expected {
		t.Errorf("Expected delay %v, got %v", expected, *delay)
	}
}

func TestParseRetryDelay_NoRetryInfo(t *testing.T) {
	// Error format without any retry information
	noRetryInfo := []byte(`{
		"error": {
			"code": 429,
			"message": "Resource has been exhausted.",
			"details": []
		}
	}`)

	delay, err := parseRetryDelay(noRetryInfo)
	if err == nil {
		t.Fatalf("Expected error for no retry info, got nil, delay: %v", delay)
	}
	if delay != nil {
		t.Errorf("Expected delay to be nil, got: %v", delay)
	}
}

func TestParseRetryDelay_ExponentialBackoff(t *testing.T) {
	// Test that exponential backoff values work
	baseDelay := geminiRetryBaseDelay
	maxDelay := geminiRetryMaxDelay

	// Verify constants are set correctly
	if baseDelay != 1*time.Second {
		t.Errorf("Expected base delay 1s, got %v", baseDelay)
	}
	if maxDelay != 30*time.Second {
		t.Errorf("Expected max delay 30s, got %v", maxDelay)
	}

	// Test exponential backoff calculation
	delays := []time.Duration{
		baseDelay * 1,  // 1s
		baseDelay * 2,  // 2s
		baseDelay * 4,  // 4s
		baseDelay * 8,  // 8s
	}
	for i, expected := range delays {
		actual := baseDelay * time.Duration(1<<i)
		if actual != expected {
			t.Errorf("Expected delay %v for attempt %d, got %v", expected, i, actual)
		}
	}
}

func TestCliPreviewFallbackOrder(t *testing.T) {
	tests := []struct {
		model     string
		expected  []string
	}{
		{"gemini-2.5-pro", nil},        // No preview fallbacks
		{"gemini-2.5-flash", nil},      // No preview fallbacks
		{"gemini-2.5-flash-lite", nil}, // No preview fallbacks
		{"gemini-1.5-pro", nil},        // Unknown model returns nil
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			result := cliPreviewFallbackOrder(tt.model)
			if len(result) != len(tt.expected) {
				t.Errorf("cliPreviewFallbackOrder(%s) = %v, want %v", tt.model, result, tt.expected)
			}
		})
	}
}

func TestRetryLogicFlow(t *testing.T) {
	// Simulate the retry flow for a 429 with no fallback models
	// models := []string{"gemini-1.5-pro"} // Single model, no fallbacks (unused but shows the scenario)

	maxAttempts := geminiRetryMaxAttempts
	baseDelay := geminiRetryBaseDelay

	// Track expected delays
	expectedDelays := []time.Duration{
		1 * time.Second, // attempt 0: 1s (1 << 0)
		2 * time.Second, // attempt 1: 2s (1 << 1)
		4 * time.Second, // attempt 2: 4s (1 << 2)
		8 * time.Second, // attempt 3: 8s (1 << 3)
	}

	for idx := 0; idx < maxAttempts; idx++ {
		// Calculate delay like the code does
		retryDelay := baseDelay * time.Duration(1<<idx)
		if retryDelay > geminiRetryMaxDelay {
			retryDelay = geminiRetryMaxDelay
		}

		expected := expectedDelays[idx]
		if idx >= len(expectedDelays) {
			expected = geminiRetryMaxDelay
		}

		if retryDelay != expected {
			t.Errorf("Attempt %d: expected delay %v, got %v", idx, expected, retryDelay)
		}

		// Check if retry should continue
		if idx < maxAttempts-1 {
			// Should continue (retry)
		} else {
			// Should stop (max attempts reached)
		}
	}
}
