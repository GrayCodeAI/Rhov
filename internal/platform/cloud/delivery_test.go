package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func validDeliveryContext() DeliveryContext {
	event := DeliveryContext{ProjectID: "project_0123456789", Branch: "main", CommitSHA: "abc123"}
	event.Repository.Provider, event.Repository.ExternalID, event.Repository.Name = "github", "GrayCodeAI/rho", "GrayCodeAI/rho"
	return event
}

func TestSendDeliveryContextReportsRejection(t *testing.T) {
	server := httptest.NewServer(respondWith(http.StatusBadRequest, `{"error":"Invalid delivery context"}`))
	defer server.Close()
	err := New(Config{Endpoint: server.URL, DeviceToken: "hwc_test"}).SendDeliveryContext(context.Background(), validDeliveryContext())
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest || !strings.Contains(err.Error(), "Invalid delivery context") {
		t.Fatalf("error = %v, want the Worker's 400", err)
	}

	revoked := httptest.NewServer(respondWith(http.StatusUnauthorized, `{"error":"Unauthorized"}`))
	defer revoked.Close()
	err = New(Config{Endpoint: revoked.URL, DeviceToken: "hwc_revoked"}).SendDeliveryContext(context.Background(), validDeliveryContext())
	if err == nil || !strings.Contains(err.Error(), "rho cloud login") {
		t.Fatalf("revoked-token error = %v, want a reconnect hint", err)
	}
}

func TestSendDeliveryContextValidatesBeforeSending(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	client := New(Config{Endpoint: server.URL, DeviceToken: "hwc_test"})

	cases := map[string]func(*DeliveryContext){
		"short project id": func(e *DeliveryContext) { e.ProjectID = "short" },
		"bad ci status": func(e *DeliveryContext) {
			e.CIRun = &CIRunContext{Provider: "github", ExternalID: "1", Status: "passed"}
		},
		"bad deploy status": func(e *DeliveryContext) {
			e.Deployment = &DeploymentContext{Provider: "github", ExternalID: "d1", Environment: "prod", Status: "done"}
		},
		"no environment": func(e *DeliveryContext) {
			e.Deployment = &DeploymentContext{Provider: "github", ExternalID: "d1", Status: "running"}
		},
		"blank repository": func(e *DeliveryContext) { e.Repository.Name = "   " },
		"long branch":      func(e *DeliveryContext) { e.Branch = strings.Repeat("b", 201) },
		"long commit":      func(e *DeliveryContext) { e.CommitSHA = strings.Repeat("c", 129) },
	}
	for name, mutate := range cases {
		event := validDeliveryContext()
		mutate(&event)
		if err := client.SendDeliveryContext(context.Background(), event); err == nil {
			t.Errorf("%s: invalid delivery context was accepted", name)
		}
	}
	if n := hits.Load(); n != 0 {
		t.Fatalf("server received %d invalid requests, want 0", n)
	}
}

func TestSendDeliveryContextTrimsFields(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	event := validDeliveryContext()
	event.Branch = "  main \n"
	event.CIRun = &CIRunContext{Provider: " github ", ExternalID: " 42 ", Status: " succeeded "}
	if err := New(Config{Endpoint: server.URL, DeviceToken: "hwc_test"}).SendDeliveryContext(context.Background(), event); err != nil {
		t.Fatal(err)
	}
	ciRun := body["ciRun"].(map[string]any)
	if body["branch"] != "main" || ciRun["status"] != "succeeded" || ciRun["externalId"] != "42" {
		t.Fatalf("body = %v", body)
	}
}

func TestValidOpaqueID(t *testing.T) {
	for id, want := range map[string]bool{
		"0123456789abcdef":                 true,
		"exec-1727400000000-ab12":          true,
		"graph_" + strings.Repeat("a", 64): true,
		"a:b.c_d-e0123456789":              true,
		"0123456789abcde":                  false,
		strings.Repeat("a", 129):           false,
		"0123456789 abcdef":                false,
		"0123456789/abcdef":                false,
	} {
		if got := ValidOpaqueID(id); got != want {
			t.Errorf("ValidOpaqueID(%q) = %v, want %v", id, got, want)
		}
	}
}
