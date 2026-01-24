// Package claude provides request translation functionality for Claude API.
// It handles parsing and transforming Claude API requests into the internal client format,
// extracting model information, system instructions, message contents, and tool declarations.
// The package also performs JSON data cleaning and transformation to ensure compatibility
// between Claude API format and the internal client's expected format.
package claude

import (
	"bytes"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/gemini/common"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/translator/translator"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ConvertClaudeRequestToGemini parses a Claude API request and returns a complete
// Gemini CLI request body (as JSON bytes) ready to be sent via SendRawMessageStream.
// All JSON transformations are performed using gjson/sjson.
//
// Parameters:
//   - modelName: The name of the model.
//   - rawJSON: The raw JSON request from the Claude API.
//   - stream: A boolean indicating if the request is for a streaming response.
//
// Returns:
//   - []byte: The transformed request in Gemini CLI format.
func ConvertClaudeRequestToGemini(modelName string, inputRawJSON []byte, _ bool) []byte {
	rawJSON := bytes.Clone(inputRawJSON)
	rawJSON = bytes.Replace(rawJSON, []byte(`"url":{"type":"string","format":"uri",`), []byte(`"url":{"type":"string",`), -1)

	// Build output Gemini CLI request JSON
	out := `{"contents":[]}`
	out, _ = sjson.Set(out, "model", modelName)

	// system instruction
	if systemResult := gjson.GetBytes(rawJSON, "system"); systemResult.IsArray() {
		systemInstruction := `{"role":"user","parts":[]}`
		hasSystemParts := false
		systemResult.ForEach(func(_, systemPromptResult gjson.Result) bool {
			if systemPromptResult.Get("type").String() == "text" {
				textResult := systemPromptResult.Get("text")
				if textResult.Type == gjson.String {
					part := `{"text":""}`
					part, _ = sjson.Set(part, "text", textResult.String())
					systemInstruction, _ = sjson.SetRaw(systemInstruction, "parts.-1", part)
					hasSystemParts = true
				}
			}
			return true
		})
		if hasSystemParts {
			out, _ = sjson.SetRaw(out, "system_instruction", systemInstruction)
		}
	} else if systemResult.Type == gjson.String {
		out, _ = sjson.Set(out, "system_instruction.parts.-1.text", systemResult.String())
	}

	// contents
	if messagesResult := gjson.GetBytes(rawJSON, "messages"); messagesResult.IsArray() {
		messagesResult.ForEach(func(_, messageResult gjson.Result) bool {
			roleResult := messageResult.Get("role")
			if roleResult.Type != gjson.String {
				return true
			}
			role := roleResult.String()
			if role == "assistant" {
				role = "model"
			}

			contentJSON := `{"role":"","parts":[]}`
			contentJSON, _ = sjson.Set(contentJSON, "role", role)

			contentsResult := messageResult.Get("content")
			if contentsResult.IsArray() {
				contentsResult.ForEach(func(_, contentResult gjson.Result) bool {
					switch contentResult.Get("type").String() {
					case "text":
						part := `{"text":""}`
						part, _ = sjson.Set(part, "text", contentResult.Get("text").String())
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)

					case "tool_use":
						functionName := contentResult.Get("name").String()
						functionArgs := contentResult.Get("input").String()
						argsResult := gjson.Parse(functionArgs)
						if argsResult.IsObject() && gjson.Valid(functionArgs) {
							part := `{"thoughtSignature":"","functionCall":{"name":"","args":{}}}`
							part, _ = sjson.Set(part, "thoughtSignature", translator.SkipThoughtSignatureValidator)
							part, _ = sjson.Set(part, "functionCall.name", functionName)
							part, _ = sjson.SetRaw(part, "functionCall.args", functionArgs)
							contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
						}

					case "tool_result":
						toolCallID := contentResult.Get("tool_use_id").String()
						if toolCallID == "" {
							return true
						}
						funcName := toolCallID
						toolCallIDs := strings.Split(toolCallID, "-")
						if len(toolCallIDs) > 1 {
							funcName = strings.Join(toolCallIDs[0:len(toolCallIDs)-1], "-")
						}

						// Extract content from tool_result - can be string, object, or array
						contentResult := contentResult.Get("content")
						var responseData string
						if contentResult.IsArray() {
							// Handle array of content blocks - extract text from first text block
							var textContent strings.Builder
							contentResult.ForEach(func(_, block gjson.Result) bool {
								if text := block.Get("text"); text.Exists() {
									textContent.WriteString(text.String())
								}
								return true
							})
							responseData = textContent.String()
							if responseData == "" {
								responseData = contentResult.Raw // Fallback to raw JSON
							}
						} else if contentResult.Type == gjson.String {
							// Simple string content
							responseData = contentResult.String()
						} else if contentResult.IsObject() {
							// Object with potential text field
							if text := contentResult.Get("text"); text.Exists() {
								responseData = text.String()
							} else {
								responseData = contentResult.Raw // Fallback to raw JSON
							}
						} else {
							responseData = contentResult.Raw // Fallback
						}

						// Check for error in tool result
						isError := contentResult.Get("is_error").Bool()
						part := `{"functionResponse":{"name":"","response":{"result":""}}}`
						part, _ = sjson.Set(part, "functionResponse.name", funcName)
						part, _ = sjson.Set(part, "functionResponse.response.result", responseData)
						if isError {
							part, _ = sjson.Set(part, "functionResponse.response.isError", true)
						}
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
					}

					// Additional content block types that may be present
					switch contentResult.Get("type").String() {
					case "thinking":
						// Thinking blocks are internal to Claude and not passed to Gemini
						// These are handled separately in the thinking pipeline
						return true

					case "image":
						// Image content blocks - pass through as data
						// Extract image URL or base64 data
						if source := contentResult.Get("source"); source.Exists() {
							if mediaType := source.Get("media_type").String(); mediaType != "" {
								if data := source.Get("data").String(); data != "" {
									// Base64 encoded image
									part := `{"text":""}`
									part, _ = sjson.Set(part, "text", "[Image: "+mediaType+"]")
									contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
								} else if url := source.Get("url").String(); url != "" {
									// Image URL
									part := `{"text":""}`
									part, _ = sjson.Set(part, "text", "[Image: "+url+"]")
									contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
								}
							}
						}

					case "server_tool_use":
						// Server tool use (web_search, web_fetch) - convert to functionCall for Gemini
						toolName := contentResult.Get("name").String()
						toolInput := contentResult.Get("input").String()
						if toolName != "" && toolInput != "" {
							part := `{"thoughtSignature":"","functionCall":{"name":"","args":{}}}`
							part, _ = sjson.Set(part, "thoughtSignature", translator.SkipThoughtSignatureValidator)
							part, _ = sjson.Set(part, "functionCall.name", toolName)
							part, _ = sjson.SetRaw(part, "functionCall.args", toolInput)
							contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
						}

					case "web_search_tool_result":
						// Web search tool results - pass through to Gemini as function response
						// Claude API format:
						// {
						//   "type": "web_search_tool_result",
						//   "tool_use_id": "srvtoolu_xxx",
						//   "content": [
						//     { "type": "web_search_result", "url": "...", "title": "...", "encrypted_content": "...", "page_age": "..." }
						//   ] | {
						//     "type": "web_search_tool_result_error",
						//     "error_code": "..."
						//   }
						// }
						toolUseID := contentResult.Get("tool_use_id").String()
						searchContent := contentResult.Get("content")

						if toolUseID != "" {
							// Extract function name from tool_use_id
							funcName := "web_search"
							if strings.Contains(toolUseID, "-") {
								parts := strings.SplitN(toolUseID, "-", 2)
								funcName = parts[0]
							}

							part := `{"functionResponse":{"name":"","response":{"result":""}}}`
							part, _ = sjson.Set(part, "functionResponse.name", funcName)
							part, _ = sjson.Set(part, "functionResponse.id", toolUseID)

							// Pass through the full content (array of results or error object)
							if searchContent.Exists() {
								part, _ = sjson.SetRaw(part, "functionResponse.response.result", searchContent.Raw)
							}
							contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
						}

					case "web_fetch_tool_result":
						// Web fetch tool results - pass through to Gemini as function response
						// Claude API format:
						// {
						//   "type": "web_fetch_tool_result",
						//   "tool_use_id": "srvtoolu_xxx",
						//   "content": {
						//     "type": "web_fetch_result",
						//     "url": "...",
						//     "content": { "type": "document", ... },
						//     "retrieved_at": "..."
						//   } | {
						//     "type": "web_fetch_tool_error",
						//     "error_code": "..."
						//   }
						// }
						toolUseID := contentResult.Get("tool_use_id").String()
						fetchContent := contentResult.Get("content")

						if toolUseID != "" {
							// Extract function name from tool_use_id
							funcName := "web_fetch"
							if strings.Contains(toolUseID, "-") {
								parts := strings.SplitN(toolUseID, "-", 2)
								funcName = parts[0]
							}

							part := `{"functionResponse":{"name":"","response":{"result":""}}}`
							part, _ = sjson.Set(part, "functionResponse.name", funcName)
							part, _ = sjson.Set(part, "functionResponse.id", toolUseID)

							// Pass through the full content (web_fetch_result or error object)
							if fetchContent.Exists() {
								part, _ = sjson.SetRaw(part, "functionResponse.response.result", fetchContent.Raw)
							}
							contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
						}

					case "web_search":
						// Legacy web_search content block (before versioned types)
						part := `{"text":"[Web search tool]"}`
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)

					case "web_fetch":
						// Legacy web_fetch content block (before versioned types)
						part := `{"text":"[Web fetch tool]"}`
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)

					case "text_editor":
						// Text editor tool - convert to functionCall for Gemini
						// The client is responsible for implementing the actual file operations
						toolName := contentResult.Get("action").String()
						if toolName == "" {
							toolName = contentResult.Get("name").String()
						}
						if toolName == "" {
							toolName = "text_editor"
						}
						toolInput := contentResult.Get("input").String()
						part := `{"thoughtSignature":"","functionCall":{"name":"text_editor","args":{}}}`
						part, _ = sjson.Set(part, "thoughtSignature", translator.SkipThoughtSignatureValidator)
						if toolInput != "" {
							part, _ = sjson.SetRaw(part, "functionCall.args", toolInput)
						}
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)

					case "text_editor_result":
						// Text editor result - pass through as text or error
						// The actual result is executed by the client
						resultContent := contentResult.Get("content")
						if resultContent.IsObject() {
							if errorType := resultContent.Get("type").String(); errorType == "text_editor_error" {
								part := `{"text":"[Text editor error]"}`
								contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
							}
						} else if text := resultContent.Get("text").String(); text != "" {
							part := `{"text":""}`
							part, _ = sjson.Set(part, "text", text)
							contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
						}

					case "document":
						// Document content block (from web_fetch results)
						part := `{"text":"[Document content]"}`
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)

					case "collection":
						// Collection of results (from web_search)
						part := `{"text":"[Collection]"}`
						contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
					}

					return true
				})
				out, _ = sjson.SetRaw(out, "contents.-1", contentJSON)
			} else if contentsResult.Type == gjson.String {
				part := `{"text":""}`
				part, _ = sjson.Set(part, "text", contentsResult.String())
				contentJSON, _ = sjson.SetRaw(contentJSON, "parts.-1", part)
				out, _ = sjson.SetRaw(out, "contents.-1", contentJSON)
			}
			return true
		})
	}

	// tools
	// Gemini API expects format:
	// {
	//   "tools": [
	//     { "type": "function", "name": "...", "description": "...", "parameters": {...} }
	//   ]
	// }
	if toolsResult := gjson.GetBytes(rawJSON, "tools"); toolsResult.IsArray() {
		hasTools := false
		toolsResult.ForEach(func(_, toolResult gjson.Result) bool {
			toolType := toolResult.Get("type").String()

			// Special handling: map Claude web search tool to Gemini function declaration
			// Gemini doesn't have built-in web_search, so we create a function declaration for it
			// Handle both versioned (web_search_20250305) and unversioned (web_search) types for backward compatibility
			if toolType == "web_search_20250305" || toolType == "web_search" {
				if !hasTools {
					out, _ = sjson.SetRaw(out, "tools", `[]`)
					hasTools = true
				}
				// Note: search_context_size, max_uses, allowed_domains, blocked_domains, user_location
				// are server-side parameters for Anthropic, not passed to Gemini
				// Gemini handles search internally based on the query
				out, _ = sjson.SetRaw(out, "tools.-1", translator.WebSearchToolDefinition)
				return true
			}

			// Special handling: map Claude web fetch tool to Gemini function declaration
			// Gemini doesn't have built-in web_fetch, so we create a function declaration for it
			// web_fetch_20250910 requires beta header "web-fetch-2025-09-10"
			// Handle both versioned (web_fetch_20250910) and unversioned (web_fetch) types for backward compatibility
			if toolType == "web_fetch_20250910" || toolType == "web_fetch" {
				if !hasTools {
					out, _ = sjson.SetRaw(out, "tools", `[]`)
					hasTools = true
				}
				// Note: max_uses, allowed_domains, blocked_domains, citations, max_content_tokens
				// are server-side parameters for Anthropic, not passed to Gemini
				// Gemini handles fetching internally based on the URL
				out, _ = sjson.SetRaw(out, "tools.-1", translator.WebFetchToolDefinition)
				return true
			}

			// Text editor tool - client-controlled tool for file operations
			// These tools require client implementation, we just pass through the declaration
			if strings.HasPrefix(toolType, "text_editor") {
				if !hasTools {
					out, _ = sjson.SetRaw(out, "tools", `[]`)
					hasTools = true
				}
				out, _ = sjson.SetRaw(out, "tools.-1", translator.GeminiTextEditorToolDefinition)
				return true
			}

			inputSchemaResult := toolResult.Get("input_schema")
			if inputSchemaResult.Exists() && inputSchemaResult.IsObject() {
				inputSchema := inputSchemaResult.Raw
				// Build Gemini-compatible function declaration with "type": "function" and "parameters"
				tool := `{"type":"function"}`
				tool, _ = sjson.SetRaw(tool, "parameters", inputSchema)
				if name := toolResult.Get("name").String(); name != "" {
					tool, _ = sjson.Set(tool, "name", name)
				}
				if description := toolResult.Get("description").String(); description != "" {
					tool, _ = sjson.Set(tool, "description", description)
				}
				if gjson.Valid(tool) && gjson.Parse(tool).IsObject() {
					if !hasTools {
						out, _ = sjson.SetRaw(out, "tools", `[]`)
						hasTools = true
					}
					out, _ = sjson.SetRaw(out, "tools.-1", tool)
				}
			}
			return true
		})
		if !hasTools {
			out, _ = sjson.Delete(out, "tools")
		}
	}

	// tool_choice handling - map Claude tool_choice to Gemini tool_config
	if toolChoice := gjson.GetBytes(rawJSON, "tool_choice"); toolChoice.Exists() {
		tcType := toolChoice.Get("type").String()

		// Handle web_search and web_fetch tool choices (both versioned and unversioned)
		if tcType == "web_search_20250305" || tcType == "web_search" ||
			tcType == "web_fetch_20250910" || tcType == "web_fetch" {
			out, _ = sjson.SetRaw(out, "tool_config", `{"function_calling_config":{"mode":"ANY"}}`)
		} else if tcType == "auto" {
			out, _ = sjson.SetRaw(out, "tool_config", `{"function_calling_config":{"mode":"AUTO"}}`)
		} else if tcType == "none" {
			out, _ = sjson.SetRaw(out, "tool_config", `{"function_calling_config":{"mode":"NONE"}}`)
		} else if tcType == "any" {
			out, _ = sjson.SetRaw(out, "tool_config", `{"function_calling_config":{"mode":"ANY"}}`)
		} else if tcType == "tool" {
			// Specific tool choice by name
			if toolName := toolChoice.Get("name").String(); toolName != "" {
				toolConfig := `{"function_calling_config":{"mode":"ANY"}}`
				out, _ = sjson.SetRaw(out, "tool_config", toolConfig)
			}
		}
	}

	// Map Anthropic thinking -> Gemini thinkingBudget/include_thoughts when enabled
	// Translator only does format conversion, ApplyThinking handles model capability validation.
	if t := gjson.GetBytes(rawJSON, "thinking"); t.Exists() && t.IsObject() {
		if t.Get("type").String() == "enabled" {
			// Always set includeThoughts when thinking is enabled
			out, _ = sjson.Set(out, "generationConfig.thinkingConfig.includeThoughts", true)
			if b := t.Get("budget_tokens"); b.Exists() && b.Type == gjson.Number {
				budget := int(b.Int())
				out, _ = sjson.Set(out, "generationConfig.thinkingConfig.thinkingBudget", budget)
			}
		} else if t.Get("type").String() == "disabled" {
			// Explicitly disable thinking when type is "disabled"
			out, _ = sjson.Set(out, "generationConfig.thinkingConfig.includeThoughts", false)
		}
	}
	if v := gjson.GetBytes(rawJSON, "temperature"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "generationConfig.temperature", v.Num)
	}
	if v := gjson.GetBytes(rawJSON, "top_p"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "generationConfig.topP", v.Num)
	}
	if v := gjson.GetBytes(rawJSON, "top_k"); v.Exists() && v.Type == gjson.Number {
		out, _ = sjson.Set(out, "generationConfig.topK", v.Num)
	}

	result := []byte(out)
	result = common.AttachDefaultSafetySettings(result, "safetySettings")

	return result
}
