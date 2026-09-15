package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// SleepTool pauses execution for a specified duration.
type SleepTool struct{}

func (SleepTool) Name() string      { return "Sleep" }
func (SleepTool) Aliases() []string { return []string{"sleep"} }
func (SleepTool) Description() string {
	return "Pause execution for a specified number of seconds."
}

// SleepInput is the typed input for SleepTool.
type SleepInput struct {
	Seconds float64 `json:"seconds"`
}

// Schema returns the typed input schema. Parameters() delegates to it so the
// two cannot diverge.
func (SleepTool) Schema() ToolSchema {
	return ToolSchema{
		Type: "object",
		Properties: map[string]SchemaProperty{
			"seconds": {Type: "number", Description: "Duration to sleep in seconds (max 300)"},
		},
		Required: []string{"seconds"},
	}
}

func (SleepTool) Parameters() map[string]interface{} {
	return sleepSchema.ToJSONSchema()
}

// sleepSchema is the single source of truth for Sleep's input schema.
var sleepSchema = SleepTool{}.Schema()

func (SleepTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	p, err := DecodeInput[SleepInput]("Sleep", input)
	if err != nil {
		return "", err
	}
	if p.Seconds <= 0 {
		return "", fmt.Errorf("seconds must be positive")
	}
	if p.Seconds > 300 {
		p.Seconds = 300
	}

	dur := time.Duration(p.Seconds * float64(time.Second))
	select {
	case <-time.After(dur):
		return fmt.Sprintf("Slept for %.1f seconds.", p.Seconds), nil
	case <-ctx.Done():
		return "Sleep interrupted.", ctx.Err()
	}
}
