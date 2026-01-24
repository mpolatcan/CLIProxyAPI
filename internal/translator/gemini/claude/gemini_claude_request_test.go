package claude

import (
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertClaudeRequestToGemini_ToolDefinitionFormat(t *testing.T) {
	// Test that tool definitions are converted to official Gemini API format:
	// {
	//   "tools": [
	//     { "type": "function", "name": "...", "description": "...", "parameters": {...} }
	//   ]
	// }
	inputJSON := []byte(`{
		"model": "gemini-2.0-flash-exp",
		"messages": [{"role": "user", "content": "Hello"}],
		"tools": [
			{
				"name": "get_weather",
				"description": "Get the current weather",
				"input_schema": {
					"type": "object",
					"properties": {
						"location": {"type": "string", "description": "The city name"}
					},
					"required": ["location"]
				}
			}
		]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-2.0-flash-exp", inputJSON, false)
	outputStr := string(output)

	// Check tools array exists
	toolsResult := gjson.Get(outputStr, "tools")
	if !toolsResult.IsArray() {
		t.Fatal("tools should be an array")
	}

	// Check first tool has correct format
	firstTool := toolsResult.Get("0")
	if firstTool.Get("type").String() != "function" {
		t.Errorf("Expected type 'function', got '%s'", firstTool.Get("type").String())
	}
	if firstTool.Get("name").String() != "get_weather" {
		t.Errorf("Expected name 'get_weather', got '%s'", firstTool.Get("name").String())
	}
	if firstTool.Get("description").String() != "Get the current weather" {
		t.Errorf("Expected description 'Get the current weather', got '%s'", firstTool.Get("description").String())
	}

	// Check parameters (not parametersJsonSchema)
	params := firstTool.Get("parameters")
	if !params.Exists() {
		t.Fatal("parameters should exist")
	}
	if params.Get("type").String() != "object" {
		t.Errorf("Expected parameters.type 'object', got '%s'", params.Get("type").String())
	}
	if params.Get("properties.location.type").String() != "string" {
		t.Errorf("Expected location type 'string', got '%s'", params.Get("properties.location.type").String())
	}
}

func TestConvertClaudeRequestToGemini_WebSearchToolDefinition(t *testing.T) {
	// Test that web_search tool is converted correctly
	inputJSON := []byte(`{
		"model": "gemini-2.0-flash-exp",
		"messages": [{"role": "user", "content": "Search for AI"}],
		"tools": [{"type": "web_search_20250305", "name": "web_search"}]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-2.0-flash-exp", inputJSON, false)
	outputStr := string(output)

	toolsResult := gjson.Get(outputStr, "tools")
	if !toolsResult.IsArray() {
		t.Fatal("tools should be an array")
	}

	firstTool := toolsResult.Get("0")
	if firstTool.Get("type").String() != "function" {
		t.Errorf("Expected type 'function', got '%s'", firstTool.Get("type").String())
	}
	if firstTool.Get("name").String() != "web_search" {
		t.Errorf("Expected name 'web_search', got '%s'", firstTool.Get("name").String())
	}
	if firstTool.Get("parameters.properties.query.type").String() != "string" {
		t.Errorf("Expected query parameter type 'string', got '%s'", firstTool.Get("parameters.properties.query.type").String())
	}
}

func TestConvertClaudeRequestToGemini_WebFetchToolDefinition(t *testing.T) {
	// Test that web_fetch tool is converted correctly
	inputJSON := []byte(`{
		"model": "gemini-2.0-flash-exp",
		"messages": [{"role": "user", "content": "Fetch URL"}],
		"tools": [{"type": "web_fetch_20250910", "name": "web_fetch"}]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-2.0-flash-exp", inputJSON, false)
	outputStr := string(output)

	toolsResult := gjson.Get(outputStr, "tools")
	if !toolsResult.IsArray() {
		t.Fatal("tools should be an array")
	}

	firstTool := toolsResult.Get("0")
	if firstTool.Get("type").String() != "function" {
		t.Errorf("Expected type 'function', got '%s'", firstTool.Get("type").String())
	}
	if firstTool.Get("name").String() != "web_fetch" {
		t.Errorf("Expected name 'web_fetch', got '%s'", firstTool.Get("name").String())
	}
	if firstTool.Get("parameters.properties.url.type").String() != "string" {
		t.Errorf("Expected url parameter type 'string', got '%s'", firstTool.Get("parameters.properties.url.type").String())
	}
}

func TestConvertClaudeRequestToGemini_MultipleTools(t *testing.T) {
	// Test that multiple tools are converted correctly
	inputJSON := []byte(`{
		"model": "gemini-2.0-flash-exp",
		"messages": [{"role": "user", "content": "Use tools"}],
		"tools": [
			{"type": "web_search_20250305"},
			{"type": "web_fetch_20250910"}
		]
	}`)

	output := ConvertClaudeRequestToGemini("gemini-2.0-flash-exp", inputJSON, false)
	outputStr := string(output)

	toolsResult := gjson.Get(outputStr, "tools")
	if !toolsResult.IsArray() {
		t.Fatal("tools should be an array")
	}

	toolsArray := toolsResult.Array()
	if len(toolsArray) != 2 {
		t.Errorf("Expected 2 tools, got %d", len(toolsArray))
	}

	if toolsArray[0].Get("name").String() != "web_search" {
		t.Errorf("First tool should be web_search, got '%s'", toolsArray[0].Get("name").String())
	}
	if toolsArray[1].Get("name").String() != "web_fetch" {
		t.Errorf("Second tool should be web_fetch, got '%s'", toolsArray[1].Get("name").String())
	}
}
