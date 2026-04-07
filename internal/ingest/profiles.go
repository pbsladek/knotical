package ingest

import (
	"fmt"
	"slices"
	"strings"
)

type TransformSpec struct {
	Name string
	Arg  string
}

func (t TransformSpec) Key() string {
	return strings.TrimSpace(strings.ToLower(t.Name)) + "\x00" + strings.TrimSpace(t.Arg)
}

type Profile struct {
	Name        string
	Description string
	Transforms  []TransformSpec
	HeadLines   int
	TailLines   int
	MaxLines    int
	MaxTokens   int
}

type Pipeline struct {
	Profile    Profile
	Shorthands []string
	Transforms []TransformSpec
	HeadLines  int
	TailLines  int
	MaxLines   int
	MaxTokens  int
}

type PipelineOptions struct {
	Profile    string
	Shorthands []string
	Transforms []string
	NoPipeline bool
}

var builtinProfiles = []Profile{
	{
		Name:        "compact",
		Description: "Terse log summary with sanitization, dedupe, and a modest tail cap.",
		Transforms: []TransformSpec{
			{Name: "strip-ansi"},
			{Name: "strip-timestamps"},
			{Name: "unique-count"},
		},
		TailLines: 400,
	},
	{
		Name:        "k8s",
		Description: "Kubernetes-oriented normalization and deduplication.",
		Transforms: []TransformSpec{
			{Name: "strip-ansi"},
			{Name: "normalize-k8s"},
			{Name: "unique-count"},
		},
		TailLines: 400,
	},
	{
		Name:        "errors",
		Description: "Error-focused profile that keeps only likely signal lines.",
		Transforms: []TransformSpec{
			{Name: "strip-ansi"},
			{Name: "strip-timestamps"},
			{Name: "include-regex", Arg: `(?i)(error|warn|fatal|panic)`},
			{Name: "unique-count"},
		},
		TailLines: 400,
	},
	{
		Name:        "incident",
		Description: "Broader incident-analysis profile with normalization and dedupe.",
		Transforms: []TransformSpec{
			{Name: "strip-ansi"},
			{Name: "strip-timestamps"},
			{Name: "normalize-k8s"},
			{Name: "unique-count"},
		},
		TailLines: 800,
		MaxTokens: 4000,
	},
}

var shorthandExpansions = map[string][]TransformSpec{
	"clean": {
		{Name: "strip-ansi"},
		{Name: "strip-timestamps"},
	},
	"dedupe": {
		{Name: "dedupe-exact"},
	},
	"unique": {
		{Name: "unique-count"},
	},
	"k8s": {
		{Name: "normalize-k8s"},
	},
}

func BuiltinProfiles() []Profile {
	ordered := slices.Clone(builtinProfiles)
	for idx := range ordered {
		ordered[idx].Transforms = append([]TransformSpec(nil), ordered[idx].Transforms...)
	}
	slices.SortFunc(ordered, func(a, b Profile) int {
		return strings.Compare(a.Name, b.Name)
	})
	return ordered
}

func LookupProfile(name string) (Profile, bool) {
	normalized := normalizeProfileName(name)
	for _, profile := range builtinProfiles {
		if profile.Name == normalized {
			profile.Transforms = append([]TransformSpec(nil), profile.Transforms...)
			return profile, true
		}
	}
	return Profile{}, false
}

func ResolveProfile(name string) (Profile, error) {
	if strings.TrimSpace(name) == "" {
		return Profile{}, fmt.Errorf("profile name is required")
	}
	profile, ok := LookupProfile(name)
	if !ok {
		return Profile{}, fmt.Errorf("unknown log profile %q", name)
	}
	return profile, nil
}

func applyPipelineDefaults(opts Options, pipeline Pipeline) Options {
	if pipeline.HeadLines > 0 && opts.HeadLines == 0 {
		opts.HeadLines = pipeline.HeadLines
	}
	if pipeline.TailLines > 0 && opts.TailLines == 0 {
		opts.TailLines = pipeline.TailLines
	}
	if pipeline.MaxLines > 0 && opts.MaxInputLines == 0 {
		opts.MaxInputLines = pipeline.MaxLines
	}
	if pipeline.MaxTokens > 0 && opts.MaxInputTokens == 0 {
		opts.MaxInputTokens = pipeline.MaxTokens
	}
	return opts
}

func normalizeProfileName(name string) string {
	return normalizeTransformName(name)
}

func normalizeNames(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
