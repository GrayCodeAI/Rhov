package tool

import (
	"encoding/json"
	"fmt"
)

// DecodeInput unmarshals a tool's JSON input into a typed struct with a
// consistent error message. It replaces the hand-rolled json.Unmarshal blocks
// that every tool used to embed in Execute (each with its own inconsistent
// error text). Required-field and schema validation still happen in
// ValidateToolInput before dispatch; this only handles JSON decoding.
func DecodeInput[T any](toolName string, input json.RawMessage) (T, error) {
	var v T
	if len(input) == 0 {
		return v, fmt.Errorf("tool %s: missing input", toolName)
	}
	if err := json.Unmarshal(input, &v); err != nil {
		return v, fmt.Errorf("tool %s: invalid JSON input: %w", toolName, err)
	}
	return v, nil
}
