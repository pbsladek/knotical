package ingest

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pbsladek/knotical/internal/model"
)

func validateTransformConflicts(specs []TransformSpec) error {
	has := map[string]bool{}
	for _, spec := range specs {
		has[spec.Name] = true
	}
	if has["dedupe-exact"] && has["dedupe-normalized"] {
		return fmt.Errorf("dedupe-exact and dedupe-normalized cannot be used together")
	}
	if has["dedupe-exact"] && has["unique-count"] {
		return fmt.Errorf("dedupe-exact and unique-count cannot be used together")
	}
	if has["dedupe-normalized"] && has["unique-count"] {
		return fmt.Errorf("dedupe-normalized and unique-count cannot be used together")
	}
	return nil
}

func transformCategory(name string) int {
	switch name {
	case "strip-ansi", "strip-timestamps":
		return 0
	case "normalize-k8s":
		return 1
	case "include-regex", "exclude-regex":
		return 2
	case "dedupe-exact", "dedupe-normalized", "unique-count":
		return 3
	default:
		return 9
	}
}

func applyTransform(lines []string, spec TransformSpec, report *model.ReductionMetadata) ([]string, error) {
	switch spec.Name {
	case "strip-ansi":
		return stripANSILines(lines), nil
	case "strip-timestamps":
		return stripTimestampLines(lines), nil
	case "normalize-k8s":
		return normalizeK8sLines(lines), nil
	case "include-regex":
		return filterLines(lines, spec.Arg, true)
	case "exclude-regex":
		return filterLines(lines, spec.Arg, false)
	case "dedupe-exact":
		return dedupeExactLines(lines), nil
	case "dedupe-normalized":
		return dedupeNormalizedLines(lines), nil
	case "unique-count":
		out, unique := collapseUniqueLines(lines)
		if report != nil {
			report.UniqueGroups = unique
		}
		return out, nil
	default:
		return nil, fmt.Errorf("unknown transform %q", spec.Name)
	}
}

func filterLines(lines []string, expr string, include bool) ([]string, error) {
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid regex %q: %w", expr, err)
	}
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		matched := re.MatchString(line)
		if matched == include {
			out = append(out, line)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("transform %q removed all input", re.String())
	}
	return out, nil
}

func stripANSILines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = stripANSISequences(line)
	}
	return out
}

func stripTimestampLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = stripTimestampPrefix(line)
	}
	return out
}

func normalizeK8sLines(lines []string) []string {
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = normalizeK8sLine(line)
	}
	return out
}

func dedupeExactLines(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	return out
}

func dedupeNormalizedLines(lines []string) []string {
	seen := make(map[string]struct{}, len(lines))
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		key := normalizeLogKey(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, normalizeLogDisplayLine(line))
	}
	return out
}

func collapseUniqueLines(lines []string) ([]string, int) {
	if len(lines) == 0 {
		return nil, 0
	}
	counts := map[string]int{}
	order := []string{}
	for _, line := range lines {
		if _, ok := counts[line]; !ok {
			order = append(order, line)
		}
		counts[line]++
	}
	out := make([]string, 0, len(order))
	for _, line := range order {
		count := counts[line]
		if count > 1 {
			out = append(out, fmt.Sprintf("[x%d] %s", count, line))
			continue
		}
		out = append(out, line)
	}
	return out, len(order)
}

func normalizeK8sLine(line string) string {
	line = stripTimestampPrefix(stripANSISequences(line))
	line = k8sPodSuffixRE.ReplaceAllString(line, `$1-<pod>`)
	line = strings.TrimSpace(line)
	return line
}

func normalizeLogKey(line string) string {
	return strings.ToLower(normalizeLogDisplayLine(line))
}

func normalizeLogDisplayLine(line string) string {
	line = stripTimestampPrefix(stripANSISequences(line))
	line = normalizeK8sLine(line)
	line = uuidRE.ReplaceAllString(line, "<uuid>")
	line = ipv4RE.ReplaceAllString(line, "<ip>")
	line = longNumberRE.ReplaceAllString(line, "<num>")
	line = whitespaceCollapseRE.ReplaceAllString(line, " ")
	return strings.TrimSpace(line)
}
