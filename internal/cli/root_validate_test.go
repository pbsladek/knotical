package cli

import (
	"testing"

	"github.com/pbsladek/knotical/internal/app"
)

func TestValidateRootOptionsRejectsConflictingLogFlags(t *testing.T) {
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Log = true
		req.NoLog = true
	})}); err == nil {
		t.Fatal("expected conflicting logging flags to fail")
	}
}

func TestValidateRootOptionsRejectsInvalidStdinMode(t *testing.T) {
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.StdinMode = "weird" })}); err == nil {
		t.Fatal("expected invalid stdin mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.InputReduction = "mystery" })}); err == nil {
		t.Fatal("expected invalid input reduction mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.MaxInputLines = -1 })}); err == nil {
		t.Fatal("expected negative input limit to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.MaxInputTokens = -1 })}); err == nil {
		t.Fatal("expected negative token limit to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Shell = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with shell mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Code = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with code mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.DescribeShell = true
	})}); err == nil {
		t.Fatal("expected analyze-logs conflict with describe-shell mode")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Profile = "k8s" })}); err == nil {
		t.Fatal("expected profile without analyze-logs to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.AnalyzeLogs = true
		req.Profile = "k8s"
	})}); err != nil {
		t.Fatalf("expected analyze-logs profile combination to pass, got %v", err)
	}
}

func TestValidateRootOptionsRejectsNoPipelineWithExplicitPipelineFlags(t *testing.T) {
	tests := []rootOptions{
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Profile = "k8s" })},
		{Request: rootReq(func(req *app.Request) {
			req.AnalyzeLogs = true
			req.NoPipeline = true
			req.Transforms = []string{"include-regex:error"}
		})},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Clean = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Dedupe = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.Unique = true })},
		{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true; req.K8s = true })},
	}
	for _, opts := range tests {
		if err := validateRootOptions(opts); err == nil {
			t.Fatalf("expected no-pipeline conflict for opts %+v", opts)
		}
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.AnalyzeLogs = true; req.NoPipeline = true })}); err != nil {
		t.Fatalf("expected bare --no-pipeline to pass, got %v", err)
	}
}

func TestValidateRootOptionsRejectsInvalidExecuteFlags(t *testing.T) {
	if err := validateRootOptions(rootOptions{Execute: "sandbox"}); err == nil {
		t.Fatal("expected --execute without --shell to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), Execute: "weird"}); err == nil {
		t.Fatal("expected invalid execute mode to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.ForceRiskyShell = true }), Execute: "safe"}); err == nil {
		t.Fatal("expected force-risky-shell without host execute to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "runc" })}); err == nil {
		t.Fatal("expected invalid sandbox runtime to fail")
	}
	if err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "docker" }), Execute: "safe"}); err == nil {
		t.Fatal("expected sandbox options with non-sandbox execute mode to fail")
	}
}

func TestValidateRootOptionsRejectsInvalidProvider(t *testing.T) {
	err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Provider = "weird"
	})})
	if err == nil {
		t.Fatal("expected invalid provider to fail")
	}
}

func TestValidateRootOptionsAllowsKnownProvider(t *testing.T) {
	err := validateRootOptions(rootOptions{Request: rootReq(func(req *app.Request) {
		req.Provider = "anthropic"
	})})
	if err != nil {
		t.Fatalf("expected known provider to pass, got %v", err)
	}
}

func TestNormalizeRootOptionsAppliesShellAliases(t *testing.T) {
	opts := rootOptions{
		Request: rootReq(func(req *app.Request) {
			req.Shell = true
			req.SandboxImage = "alpine:3.20"
			req.SandboxNetwork = true
			req.SandboxWrite = true
		}),
		SandboxExec:   true,
		DockerRuntime: true,
	}
	if err := normalizeRootOptions(&opts); err != nil {
		t.Fatalf("normalizeRootOptions failed: %v", err)
	}
	if opts.Execute != "sandbox" {
		t.Fatalf("expected sandbox execute alias, got %q", opts.Execute)
	}
	if opts.Request.SandboxRuntime != "docker" {
		t.Fatalf("expected docker runtime alias, got %q", opts.Request.SandboxRuntime)
	}
	if err := validateRootOptions(opts); err != nil {
		t.Fatalf("validateRootOptions failed after normalization: %v", err)
	}
}

func TestNormalizeRootOptionsRejectsConflictingAliases(t *testing.T) {
	opts := rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), Execute: "host", SafeExec: true}
	if err := normalizeRootOptions(&opts); err == nil {
		t.Fatal("expected execute alias conflict")
	}

	opts = rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true; req.SandboxRuntime = "docker" }), PodmanRuntime: true}
	if err := normalizeRootOptions(&opts); err == nil {
		t.Fatal("expected runtime alias conflict")
	}
}

func TestNormalizeRootOptionsAllowsSafeAliasWithoutSandboxOptions(t *testing.T) {
	opts := rootOptions{Request: rootReq(func(req *app.Request) { req.Shell = true }), SafeExec: true}
	if err := normalizeRootOptions(&opts); err != nil {
		t.Fatalf("normalizeRootOptions failed: %v", err)
	}
	if opts.Execute != "safe" {
		t.Fatalf("expected safe execute alias, got %q", opts.Execute)
	}
	if err := validateRootOptions(opts); err != nil {
		t.Fatalf("validateRootOptions failed: %v", err)
	}
}
