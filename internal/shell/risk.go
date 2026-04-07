package shell

import (
	"regexp"
	"strings"
)

var riskPatterns = []struct {
	re     *regexp.Regexp
	reason string
}{
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])sudo([^[:alnum:]_]|$)`), reason: "uses sudo"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])rm([^[:alnum:]_]|$)`), reason: "removes files"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])mv([^[:alnum:]_]|$)`), reason: "moves or renames files"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(chmod|chown)([^[:alnum:]_]|$)`), reason: "changes permissions or ownership"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])dd([^[:alnum:]_]|$)`), reason: "writes raw block data"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(mkfs|fdisk|diskutil)([^[:alnum:]_]|$)`), reason: "modifies disks or filesystems"},
	{re: regexp.MustCompile(`(curl|wget)[^|]*\|\s*(sh|bash|zsh)`), reason: "pipes a downloaded script into a shell"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])(scp|ssh)([^[:alnum:]_]|$)`), reason: "accesses remote hosts"},
	{re: regexp.MustCompile(`(^|[^[:alnum:]_])rsync([^[:alnum:]_]|$)`), reason: "copies files, possibly to remote hosts"},
	{re: regexp.MustCompile("[|;&<>`]|\\$\\("), reason: "uses shell operators or redirection"},
}

func AnalyzeCommand(command string) RiskReport {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return RiskReport{}
	}
	report := RiskReport{}
	seen := map[string]struct{}{}
	for _, pattern := range riskPatterns {
		if pattern.re.MatchString(trimmed) {
			if _, ok := seen[pattern.reason]; ok {
				continue
			}
			seen[pattern.reason] = struct{}{}
			report.Reasons = append(report.Reasons, pattern.reason)
		}
	}
	report.HighRisk = len(report.Reasons) > 0
	return report
}
