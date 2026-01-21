// Package translator provides shared constants and utilities for request translation
// between different AI API formats.
package translator

// SkipThoughtSignatureValidator is the sentinel value used to bypass signature
// validation when no valid thinking signature is available.
const SkipThoughtSignatureValidator = "skip_thought_signature_validator"

// Tool definition constants shared across different translator implementations.
// These definitions are used to map Claude tool types to their equivalent
// function declarations in other API formats.
const (
	// WebSearchToolDefinition is the function declaration for web search tool.
	WebSearchToolDefinition = `{"name":"web_search","description":"Search the web for information","parameters":{"type":"object","properties":{"query":{"type":"string","description":"The search query"}},"required":["query"]}}`

	// WebFetchToolDefinition is the function declaration for web fetch tool.
	WebFetchToolDefinition = `{"name":"web_fetch","description":"Fetch the full content of a web page or PDF document","parameters":{"type":"object","properties":{"url":{"type":"string","description":"The URL to fetch content from"}},"required":["url"]}}`

	// TextEditorToolDefinition is the function declaration for text editor tool.
	TextEditorToolDefinition = `{"name":"text_editor","description":"Edit text in files using various operations","parameters":{"type":"object","properties":{"command":{"type":"string","description":"The edit command: insert, delete, replace, view"},"path":{"type":"string","description":"File path to edit"},"text":{"type":"string","description":"Text to insert or replace with"},"old_text":{"type":"string","description":"Text to replace"},"offset":{"type":"integer","description":"Character offset"},"limit":{"type":"integer","description":"Number of characters to delete"}},"required":["command","path"]}}`

	// GeminiTextEditorToolDefinition is a simplified version of text editor tool for Gemini API.
	GeminiTextEditorToolDefinition = `{"name":"text_editor","description":"Edit text in files using various operations","parameters":{"type":"object","properties":{"command":{"type":"string","description":"The edit command: insert, delete, replace, view"},"path":{"type":"string","description":"File path to edit"},"text":{"type":"string","description":"Text to insert or replace with"}},"required":["command","path"]}}`
)
