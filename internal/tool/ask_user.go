package tool

import (
	"context"
	"encoding/json"
	"fmt"
)

type AskUserQuestionTool struct{}

func (AskUserQuestionTool) Name() string      { return "AskUserQuestion" }
func (AskUserQuestionTool) Aliases() []string { return []string{"ask_user"} }
func (AskUserQuestionTool) Description() string {
	return "Ask the user a clarifying question when you need more information to proceed."
}

func (AskUserQuestionTool) Parameters() map[string]interface{} {
	return askUserSchema.ToJSONSchema()
}

// askUserSchema is the single source of truth for AskUserQuestion's input
// schema. The "options" Items entry preserves the existing array-of-string
// wire shape.
var askUserSchema = AskUserQuestionTool{}.Schema()

// Schema returns the typed input schema for AskUserQuestion.
func (AskUserQuestionTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"question":       {Type: "string", Description: "The question to ask"},
			"options":        {Type: "array", Description: "Optional list of choices (for single-select)", Items: &SchemaProperty{Type: "string"}},
			"multi_select":   {Type: "boolean", Description: "Allow multiple selections (default: false)"},
			"other":          {Type: "boolean", Description: "Allow free-text 'other' option (default: true)"},
			"cancel_message": {Type: "string", Description: "Message to show on cancel"},
		},
		Required: []string{"question"},
	}
}

// AskUserInput represents structured input for ask_user tool
type AskUserInput struct {
	Question      string   `json:"question"`
	Options       []string `json:"options,omitempty"`
	MultiSelect   bool     `json:"multi_select,omitempty"`
	Other         *bool    `json:"other,omitempty"`
	CancelMessage string   `json:"cancel_message,omitempty"`
}

func (AskUserQuestionTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	p, err := DecodeInput[AskUserInput]("AskUserQuestion", input)
	if err != nil {
		return "", err
	}
	if p.Question == "" {
		return "", fmt.Errorf("question is required")
	}
	tc := GetToolContext(ctx)
	if tc == nil || tc.AskUserFn == nil {
		return "", fmt.Errorf("ask_user not configured")
	}
	if len(p.Options) > 0 {
		// Structured question with options
		return tc.AskUserFn(fmt.Sprintf("%s\nOptions: %v", p.Question, p.Options))
	}
	return tc.AskUserFn(p.Question)
}
