package ingest

import (
	"regexp"
	"strings"
)

var (
	ansiEscapeRE         = regexp.MustCompile(`\x1b\[[0-9;?]*[[:alpha:]]`)
	ansiOperatingRE      = regexp.MustCompile(`\x1b\][^\a]*\a`)
	rfc3339PrefixRE      = regexp.MustCompile(`^\s*\[?\d{4}-\d{2}-\d{2}[T ][^ ]+\]?\s*`)
	syslogPrefixRE       = regexp.MustCompile(`^\s*[A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2}\s+`)
	epochPrefixRE        = regexp.MustCompile(`^\s*\d{10}(?:\.\d+)?\s+`)
	k8sPodSuffixRE       = regexp.MustCompile(`\b([a-z0-9]([a-z0-9-]*[a-z0-9])?)-[a-f0-9]{8,10}(?:-[a-z0-9]{5})?\b`)
	uuidRE               = regexp.MustCompile(`\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	ipv4RE               = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
	longNumberRE         = regexp.MustCompile(`\b\d{5,}\b`)
	whitespaceCollapseRE = regexp.MustCompile(`\s+`)
)

func stripANSISequences(line string) string {
	line = ansiEscapeRE.ReplaceAllString(line, "")
	line = ansiOperatingRE.ReplaceAllString(line, "")
	return line
}

func stripTimestampPrefix(line string) string {
	line = rfc3339PrefixRE.ReplaceAllString(line, "")
	line = syslogPrefixRE.ReplaceAllString(line, "")
	line = epochPrefixRE.ReplaceAllString(line, "")
	return strings.TrimSpace(line)
}
