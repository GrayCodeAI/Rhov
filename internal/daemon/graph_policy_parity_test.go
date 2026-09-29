package daemon

import (
	"encoding/json"
	"os"
	"testing"
)

// TestGraphAttributePolicyMatchesCloud keeps the daemon's mirror of the cloud
// sensitive-attribute policy on the shared fixture the cloud client tests use.
func TestGraphAttributePolicyMatchesCloud(t *testing.T) {
	raw, err := os.ReadFile("../platform/cloud/testdata/graph_attribute_policy.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Key       string `json:"key"`
			CloudSafe bool   `json:"cloudSafe"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) == 0 {
		t.Fatal("empty policy fixture")
	}
	for _, tc := range fixture.Cases {
		if unsafe := hasUnsafeAttributes(map[string]string{tc.Key: "v"}); unsafe == tc.CloudSafe {
			t.Errorf("key %q: daemon unsafe = %v, want cloud-safe %v", tc.Key, unsafe, tc.CloudSafe)
		}
	}
}
