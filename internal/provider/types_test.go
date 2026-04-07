package provider

import "testing"

func TestValidateBaseURLRejectsInsecureRemoteCloudEndpoints(t *testing.T) {
	for _, providerName := range []string{"openai", "anthropic", "gemini"} {
		if err := validateBaseURL(providerName, "http://example.com"); err == nil {
			t.Fatalf("expected insecure remote %s base URL to fail", providerName)
		}
	}
}

func TestValidateBaseURLAllowsLocalHTTPAndHTTPS(t *testing.T) {
	cases := []struct {
		providerName string
		rawURL       string
	}{
		{providerName: "openai", rawURL: "http://127.0.0.1:8080"},
		{providerName: "anthropic", rawURL: "http://localhost:8080"},
		{providerName: "gemini", rawURL: "https://example.com"},
		{providerName: "ollama", rawURL: "http://example.com"},
	}
	for _, tc := range cases {
		if err := validateBaseURL(tc.providerName, tc.rawURL); err != nil {
			t.Fatalf("expected %s base URL %q to pass, got %v", tc.providerName, tc.rawURL, err)
		}
	}
}

func TestValidateBaseURLRejectsCredentialsAndInvalidSchemes(t *testing.T) {
	if err := validateBaseURL("openai", "https://user:pass@example.com"); err == nil {
		t.Fatal("expected credentials in base URL to fail")
	}
	if err := validateBaseURL("openai", "ftp://example.com"); err == nil {
		t.Fatal("expected unsupported scheme to fail")
	}
	if err := validateBaseURL("openai", "not a url"); err == nil {
		t.Fatal("expected malformed URL to fail")
	}
}

func TestCapabilitiesForTransport(t *testing.T) {
	apiCaps := CapabilitiesForTransport("openai", "api")
	if !apiCaps.NativeStreaming || !apiCaps.NativeConversation || !apiCaps.NativeSchema || !apiCaps.ModelListing {
		t.Fatalf("unexpected api capabilities: %+v", apiCaps)
	}
	cliCaps := CapabilitiesForTransport("openai", "cli")
	if cliCaps.NativeStreaming || cliCaps.NativeConversation || cliCaps.NativeSchema || cliCaps.ModelListing {
		t.Fatalf("unexpected cli capabilities: %+v", cliCaps)
	}
}

func TestResolveModelUsesExplicitProvider(t *testing.T) {
	providerName, modelID, err := ResolveModel("custom-model", "gemini", "openai")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if providerName != "gemini" || modelID != "custom-model" {
		t.Fatalf("unexpected resolution: provider=%q model=%q", providerName, modelID)
	}
}

func TestResolveModelSupportsProviderPrefixedModels(t *testing.T) {
	providerName, modelID, err := ResolveModel("anthropic/claude-sonnet-4-5", "", "openai")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if providerName != "anthropic" || modelID != "claude-sonnet-4-5" {
		t.Fatalf("unexpected resolution: provider=%q model=%q", providerName, modelID)
	}
}

func TestResolveModelRejectsConflictingExplicitProvider(t *testing.T) {
	_, _, err := ResolveModel("openai/gpt-4o-mini", "gemini", "openai")
	if err == nil {
		t.Fatal("expected conflicting provider error")
	}
}

func TestResolveModelFallsBackToHeuristicAndDefault(t *testing.T) {
	providerName, modelID, err := ResolveModel("claude-sonnet-4-5", "", "openai")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if providerName != "anthropic" || modelID != "claude-sonnet-4-5" {
		t.Fatalf("unexpected heuristic resolution: provider=%q model=%q", providerName, modelID)
	}

	providerName, modelID, err = ResolveModel("my-custom-model", "", "gemini")
	if err != nil {
		t.Fatalf("ResolveModel failed: %v", err)
	}
	if providerName != "gemini" || modelID != "my-custom-model" {
		t.Fatalf("unexpected default resolution: provider=%q model=%q", providerName, modelID)
	}
}
