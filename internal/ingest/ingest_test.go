package ingest

import "testing"

func TestProcessUsesInstructionOnlyWithoutStdin(t *testing.T) {
	got, err := Process(Options{InstructionText: "analyze"})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "analyze" {
		t.Fatalf("unexpected prompt: %q", got.PromptText)
	}
}

func TestProcessUsesReducedStdinOnly(t *testing.T) {
	got, err := Process(Options{
		StdinText:       "one\ntwo\nthree\n",
		MaxInputLines:   2,
		StdinMode:       ModeAuto,
		InstructionText: "",
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "one\ntwo" {
		t.Fatalf("unexpected prompt: %q", got.PromptText)
	}
}

func TestProcessCombinesInstructionAndReducedStdin(t *testing.T) {
	got, err := Process(Options{
		InstructionText: "find the root cause",
		StdinText:       "a\nb\nc\nd\n",
		HeadLines:       1,
		TailLines:       1,
		StdinLabel:      "logs",
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	want := "find the root cause\n\nlogs:\na\nd"
	if got.PromptText != want {
		t.Fatalf("unexpected prompt:\nwant: %q\ngot:  %q", want, got.PromptText)
	}
}

func TestProcessTruncatesByBytes(t *testing.T) {
	got, err := Process(Options{
		StdinText:     "abcdefg",
		MaxInputBytes: 4,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "abcd" {
		t.Fatalf("unexpected prompt: %q", got.PromptText)
	}
}

func TestProcessSamplesDeterministically(t *testing.T) {
	got, err := Process(Options{
		StdinText:   "l1\nl2\nl3\nl4\nl5\n",
		SampleLines: 3,
		StdinMode:   ModeReplace,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "l1\nl3\nl5" {
		t.Fatalf("unexpected sampled prompt: %q", got.PromptText)
	}
}

func TestProcessReplaceModeRequiresStdin(t *testing.T) {
	_, err := Process(Options{
		InstructionText: "ignored",
		StdinMode:       ModeReplace,
	})
	if err == nil || err.Error() != "--stdin-mode replace requires piped stdin" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEstimateTokensUsesStableHeuristic(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("unexpected empty token estimate: %d", got)
	}
	if got := EstimateTokens("abcd"); got != 1 {
		t.Fatalf("unexpected token estimate: %d", got)
	}
	if got := EstimateTokens("abcde"); got != 2 {
		t.Fatalf("unexpected token estimate: %d", got)
	}
}

func TestProcessTruncatesToTokenBudget(t *testing.T) {
	got, err := Process(Options{
		StdinText:      "abcdefghijklmno",
		StdinMode:      ModeReplace,
		MaxInputTokens: 2,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "abcdefgh" {
		t.Fatalf("unexpected token-truncated prompt: %q", got.PromptText)
	}
	if EstimateTokens(got.PromptText) > 2 {
		t.Fatalf("expected prompt to fit token budget, got %d", EstimateTokens(got.PromptText))
	}
}

func TestProcessFailsOnOversizeWhenRequested(t *testing.T) {
	_, err := Process(Options{
		StdinText:      "abcdefghijklmno",
		StdinMode:      ModeReplace,
		MaxInputTokens: 2,
		InputReduction: ReductionFail,
	})
	if err == nil || err.Error() != "input exceeds max token budget: estimated 4 > 2" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProcessLeavesOversizeInputWhenReductionOff(t *testing.T) {
	got, err := Process(Options{
		StdinText:      "abcdefghijklmno",
		StdinMode:      ModeReplace,
		MaxInputTokens: 2,
		InputReduction: ReductionOff,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if got.PromptText != "abcdefghijklmno" {
		t.Fatalf("unexpected prompt: %q", got.PromptText)
	}
}

func TestProcessMarksOversizeInputForSummarization(t *testing.T) {
	got, err := Process(Options{
		StdinText:      "abcdefghijklmno",
		StdinMode:      ModeReplace,
		MaxInputTokens: 2,
		InputReduction: ReductionSummarize,
	})
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if !got.NeedsSummarization {
		t.Fatal("expected summarization to be requested")
	}
	if got.Reduction == nil || got.Reduction.Mode != ReductionSummarize {
		t.Fatalf("expected summarize reduction metadata, got %+v", got.Reduction)
	}
	if got.PromptText != "abcdefghijklmno" {
		t.Fatalf("expected prompt to remain intact before summarization, got %q", got.PromptText)
	}
}
