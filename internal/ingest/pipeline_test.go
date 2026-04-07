package ingest

import (
	"strings"
	"testing"

	"github.com/pbsladek/knotical/internal/model"
)

func TestBuiltinProfilesExposeExpectedDefaults(t *testing.T) {
	profile, ok := LookupProfile("k8s")
	if !ok {
		t.Fatal("expected k8s profile to exist")
	}
	if profile.TailLines != 400 {
		t.Fatalf("unexpected k8s tail default: %d", profile.TailLines)
	}
	if len(profile.Transforms) < 2 {
		t.Fatalf("expected k8s profile transforms, got %+v", profile)
	}
}

func TestResolvePipelineExpandsProfileAndShorthand(t *testing.T) {
	pipeline, err := ResolvePipeline(PipelineOptions{
		Profile:    "compact",
		Shorthands: []string{"clean"},
		Transforms: []string{"include-regex:warning"},
	})
	if err != nil {
		t.Fatalf("ResolvePipeline failed: %v", err)
	}
	if pipeline.Profile.Name != "compact" {
		t.Fatalf("unexpected profile: %+v", pipeline.Profile)
	}
	if pipeline.TailLines != 400 {
		t.Fatalf("expected profile tail default, got %d", pipeline.TailLines)
	}
	if len(pipeline.Transforms) != 4 {
		t.Fatalf("expected duplicate sanitizers to collapse, got %+v", pipeline.Transforms)
	}
	if pipeline.Transforms[0].Name != "strip-ansi" || pipeline.Transforms[1].Name != "strip-timestamps" || pipeline.Transforms[2].Name != "include-regex" || pipeline.Transforms[3].Name != "unique-count" {
		t.Fatalf("unexpected transform order: %+v", pipeline.Transforms)
	}
}

func TestResolvePipelineRejectsConflictingCollapseTransforms(t *testing.T) {
	_, err := ResolvePipeline(PipelineOptions{
		Transforms: []string{"dedupe-exact", "unique-count"},
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}

func TestResolvePipelineRejectsUnknownProfile(t *testing.T) {
	_, err := ResolvePipeline(PipelineOptions{Profile: "mystery"})
	if err == nil {
		t.Fatal("expected unknown profile error")
	}
}

func TestApplyPipelineDedupesExactLines(t *testing.T) {
	out, err := ApplyPipeline("a\na\nb\n", Pipeline{
		Transforms: []TransformSpec{{Name: "dedupe-exact"}},
	}, &model.ReductionMetadata{})
	if err != nil {
		t.Fatalf("applyPipeline failed: %v", err)
	}
	if out != "a\nb" {
		t.Fatalf("unexpected deduped output: %q", out)
	}
}

func TestApplyPipelineUniqueCountCollapsesDuplicates(t *testing.T) {
	report := &model.ReductionMetadata{}
	out, err := ApplyPipeline("a\na\nb\n", Pipeline{
		Transforms: []TransformSpec{{Name: "unique-count"}},
	}, report)
	if err != nil {
		t.Fatalf("applyPipeline failed: %v", err)
	}
	if out != "[x2] a\nb" {
		t.Fatalf("unexpected unique-count output: %q", out)
	}
	if report.UniqueGroups != 2 {
		t.Fatalf("expected unique group count, got %d", report.UniqueGroups)
	}
}

func TestApplyPipelineStripsNoiseBeforeDedupingNormalized(t *testing.T) {
	out, err := ApplyPipeline(strings.Join([]string{
		"2026-04-04T10:00:00Z pod/api-1234567890-abcde error request_id=123e4567-e89b-12d3-a456-426614174000",
		"2026-04-04T10:01:00Z pod/api-abcdef1234-abcde error request_id=00000000-0000-0000-0000-000000000000",
	}, "\n"), Pipeline{
		Transforms: []TransformSpec{{Name: "strip-timestamps"}, {Name: "normalize-k8s"}, {Name: "dedupe-normalized"}},
	}, &model.ReductionMetadata{})
	if err != nil {
		t.Fatalf("applyPipeline failed: %v", err)
	}
	if out != "pod/api-<pod> error request_id=<uuid>" {
		t.Fatalf("unexpected normalized output: %q", out)
	}
}

func TestApplyPipelineRejectsInvalidRegex(t *testing.T) {
	_, err := ApplyPipeline("a\nb\n", Pipeline{
		Transforms: []TransformSpec{{Name: "include-regex", Arg: "("}},
	}, &model.ReductionMetadata{})
	if err == nil {
		t.Fatal("expected invalid regex error")
	}
}

func TestProcessAppliesProfilePipeline(t *testing.T) {
	got, err := Process(Options{
		StdinText:      "2026-04-04T10:00:00Z pod/api-1234567890-abcde error\n2026-04-04T10:00:00Z pod/api-1234567890-abcde error\n",
		StdinMode:      ModeReplace,
		Profile:        "k8s",
		Shorthands:     []string{"clean"},
		InputReduction: ReductionTruncate,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.Reduction == nil || got.Reduction.Profile != "k8s" {
		t.Fatalf("expected profile metadata, got %+v", got.Reduction)
	}
	if !strings.Contains(got.PromptText, "[x2] pod/api-<pod> error") {
		t.Fatalf("expected normalized prompt text, got %q", got.PromptText)
	}
	if got.Reduction == nil || !got.Reduction.PipelineApplied {
		t.Fatalf("expected pipeline metadata to be recorded, got %+v", got.Reduction)
	}
}
