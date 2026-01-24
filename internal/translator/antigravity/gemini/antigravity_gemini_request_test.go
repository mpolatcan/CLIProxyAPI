package gemini

import (
	"fmt"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
	"github.com/tidwall/gjson"
)

func TestConvertGeminiRequestToAntigravity_PreserveValidSignature(t *testing.T) {
	// Valid signature on functionCall should be preserved
	validSignature := "abc123validSignature1234567890123456789012345678901234567890"
	inputJSON := []byte(fmt.Sprintf(`{
		"model": "gemini-3-pro-preview",
		"contents": [
			{
				"role": "model",
				"parts": [
					{"functionCall": {"name": "test_tool", "args": {}}, "thoughtSignature": "%s"}
				]
			}
		]
	}`, validSignature))

	output := ConvertGeminiRequestToAntigravity("gemini-3-pro-preview", inputJSON, false)
	outputStr := string(output)

	// Check that valid thoughtSignature is preserved
	parts := gjson.Get(outputStr, "request.contents.0.parts").Array()
	if len(parts) != 1 {
		t.Fatalf("Expected 1 part, got %d", len(parts))
	}

	sig := parts[0].Get("thoughtSignature").String()
	if sig != validSignature {
		t.Errorf("Expected thoughtSignature '%s', got '%s'", validSignature, sig)
	}
}

func TestConvertGeminiRequestToAntigravity_AddSkipSentinelToFunctionCall(t *testing.T) {
	// functionCall without signature should get skip_thought_signature_validator
	inputJSON := []byte(`{
		"model": "gemini-3-pro-preview",
		"contents": [
			{
				"role": "model",
				"parts": [
					{"functionCall": {"name": "test_tool", "args": {}}}
				]
			}
		]
	}`)

	output := ConvertGeminiRequestToAntigravity("gemini-3-pro-preview", inputJSON, false)
	outputStr := string(output)

	// Check that skip sentinel is added to functionCall
	sig := gjson.Get(outputStr, "request.contents.0.parts.0.thoughtSignature").String()
	expectedSig := translator.SkipThoughtSignatureValidator
	if sig != expectedSig {
		t.Errorf("Expected skip sentinel '%s', got '%s'", expectedSig, sig)
	}
}

func TestConvertGeminiRequestToAntigravity_RemoveThinkingBlocks(t *testing.T) {
	// Note: Thinking blocks are NOT removed - they are annotated with skip_thought_signature_validator
	// to bypass signature validation. See implementation comment: "we keep the parts; we only annotate them"
	validSignature := "abc123validSignature1234567890123456789012345678901234567890"
	inputJSON := []byte(fmt.Sprintf(`{
		"model": "gemini-3-pro-preview",
		"contents": [
			{
				"role": "model",
				"parts": [
					{"thought": true, "text": "Thinking...", "thoughtSignature": "%s"},
					{"text": "Here is my response"}
				]
			}
		]
	}`, validSignature))

	output := ConvertGeminiRequestToAntigravity("gemini-3-pro-preview", inputJSON, false)
	outputStr := string(output)

	// Check that thinking block is annotated with skip sentinel (not removed)
	parts := gjson.Get(outputStr, "request.contents.0.parts").Array()
	if len(parts) != 2 {
		t.Fatalf("Expected 2 parts (thinking block annotated, not removed), got %d", len(parts))
	}

	// First part should be the thinking block with skip sentinel
	if !parts[0].Get("thought").Bool() {
		t.Error("First part should be thinking block")
	}
	expectedSig := translator.SkipThoughtSignatureValidator
	if sig := parts[0].Get("thoughtSignature").String(); sig != expectedSig {
		t.Errorf("Expected thoughtSignature '%s', got '%s'", expectedSig, sig)
	}
	if text := parts[0].Get("text").String(); text != "Thinking..." {
		t.Errorf("Expected thinking text 'Thinking...', got '%s'", text)
	}

	// Second part should be the regular text response
	if parts[1].Get("thought").Bool() {
		t.Error("Second part should not be thinking block")
	}
	if parts[1].Get("text").String() != "Here is my response" {
		t.Errorf("Expected text 'Here is my response', got '%s'", parts[1].Get("text").String())
	}
}

func TestConvertGeminiRequestToAntigravity_ParallelFunctionCalls(t *testing.T) {
	// Multiple functionCalls should all get skip_thought_signature_validator
	inputJSON := []byte(`{
		"model": "gemini-3-pro-preview",
		"contents": [
			{
				"role": "model",
				"parts": [
					{"functionCall": {"name": "tool_one", "args": {"a": "1"}}},
					{"functionCall": {"name": "tool_two", "args": {"b": "2"}}}
				]
			}
		]
	}`)

	output := ConvertGeminiRequestToAntigravity("gemini-3-pro-preview", inputJSON, false)
	outputStr := string(output)

	parts := gjson.Get(outputStr, "request.contents.0.parts").Array()
	if len(parts) != 2 {
		t.Fatalf("Expected 2 parts, got %d", len(parts))
	}

	expectedSig := translator.SkipThoughtSignatureValidator
	for i, part := range parts {
		sig := part.Get("thoughtSignature").String()
		if sig != expectedSig {
			t.Errorf("Part %d: Expected '%s', got '%s'", i, expectedSig, sig)
		}
	}
}
