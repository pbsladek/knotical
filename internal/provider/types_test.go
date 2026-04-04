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
