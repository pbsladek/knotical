package ingest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
)

func ResolvePipeline(opts PipelineOptions) (Pipeline, error) {
	if opts.NoPipeline {
		return Pipeline{}, nil
	}
	var pipeline Pipeline
	if strings.TrimSpace(opts.Profile) != "" {
		profile, err := ResolveProfile(opts.Profile)
		if err != nil {
			return Pipeline{}, err
		}
		pipeline.Profile = profile
		pipeline.HeadLines = profile.HeadLines
		pipeline.TailLines = profile.TailLines
		pipeline.MaxLines = profile.MaxLines
		pipeline.MaxTokens = profile.MaxTokens
		pipeline.Transforms = append(pipeline.Transforms, profile.Transforms...)
	}
	pipeline.Shorthands = append(pipeline.Shorthands, normalizeNames(opts.Shorthands)...)
	for _, shorthand := range pipeline.Shorthands {
		steps, ok := shorthandExpansions[shorthand]
		if !ok {
			return Pipeline{}, fmt.Errorf("unknown log shorthand %q", shorthand)
		}
		pipeline.Transforms = append(pipeline.Transforms, steps...)
	}
	rawTransforms, err := ParseTransformSpecs(opts.Transforms)
	if err != nil {
		return Pipeline{}, err
	}
	pipeline.Transforms = append(pipeline.Transforms, rawTransforms...)
	pipeline.Transforms, err = normalizeTransformSpecs(pipeline.Transforms)
	if err != nil {
		return Pipeline{}, err
	}
	return pipeline, nil
}

func ParseTransformSpecs(values []string) ([]TransformSpec, error) {
	specs := make([]TransformSpec, 0, len(values))
	for _, value := range values {
		spec, err := ParseTransformSpec(value)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

func ParseTransformSpec(value string) (TransformSpec, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return TransformSpec{}, fmt.Errorf("transform name is required")
	}
	name, arg, ok := strings.Cut(value, ":")
	if !ok {
		return TransformSpec{Name: normalizeTransformName(value)}, nil
	}
	return TransformSpec{Name: normalizeTransformName(name), Arg: strings.TrimSpace(arg)}, nil
}

func ApplyPipeline(text string, pipeline Pipeline, report *model.ReductionMetadata) (string, error) {
	lines := splitLines(text)
	if len(lines) == 0 {
		return "", nil
	}
	if report != nil {
		report.PipelineApplied = len(pipeline.Transforms) > 0 || pipeline.Profile.Name != ""
		if pipeline.Profile.Name != "" {
			report.Profile = pipeline.Profile.Name
			report.Steps = append(report.Steps, "profile:"+pipeline.Profile.Name)
		}
		if len(pipeline.Shorthands) > 0 {
			report.Shorthands = append([]string(nil), pipeline.Shorthands...)
		}
		if len(pipeline.Transforms) > 0 {
			report.Transforms = transformSpecNames(pipeline.Transforms)
		}
	}
	lines, err := applyPipelineTransforms(lines, pipeline.Transforms, report)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}

func ApplyPipelineLines(lines []string, pipeline Pipeline, report *model.ReductionMetadata) ([]string, error) {
	return applyPipelineTransforms(lines, pipeline.Transforms, report)
}

func applyPipelineTransforms(lines []string, specs []TransformSpec, report *model.ReductionMetadata) ([]string, error) {
	if len(specs) == 0 || len(lines) == 0 {
		return lines, nil
	}
	ordered, err := normalizeTransformSpecs(specs)
	if err != nil {
		return nil, err
	}
	current := append([]string(nil), lines...)
	for _, spec := range ordered {
		before := len(current)
		current, err = applyTransform(current, spec, report)
		if err != nil {
			return nil, err
		}
		if report != nil {
			step := spec.Name
			if spec.Arg != "" {
				step += ":" + spec.Arg
			}
			report.Steps = append(report.Steps, "transform:"+step)
		}
		if report != nil && len(current) < before {
			report.DroppedLines += before - len(current)
		}
	}
	return current, nil
}

func normalizeTransformSpecs(specs []TransformSpec) ([]TransformSpec, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	seen := map[string]struct{}{}
	ordered := make([]TransformSpec, 0, len(specs))
	for _, spec := range specs {
		spec.Name = normalizeTransformName(spec.Name)
		spec.Arg = strings.TrimSpace(spec.Arg)
		if spec.Name == "" {
			return nil, fmt.Errorf("transform name is required")
		}
		key := spec.Key()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		ordered = append(ordered, spec)
	}
	if err := validateTransformConflicts(ordered); err != nil {
		return nil, err
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		return transformCategory(ordered[i].Name) < transformCategory(ordered[j].Name)
	})
	return ordered, nil
}

func normalizeTransformName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func transformSpecNames(specs []TransformSpec) []string {
	out := make([]string, 0, len(specs))
	for _, spec := range specs {
		if spec.Arg == "" {
			out = append(out, spec.Name)
			continue
		}
		out = append(out, spec.Name+":"+spec.Arg)
	}
	return out
}
