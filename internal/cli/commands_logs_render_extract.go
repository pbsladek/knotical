package cli

import "strings"

func extractLogCodeBlock(text string, last bool) string {
	lines := strings.Split(text, "\n")
	blocks := []string{}
	inBlock := false
	current := []string{}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			if inBlock {
				blocks = append(blocks, strings.Join(current, "\n"))
				current = nil
				inBlock = false
				continue
			}
			inBlock = true
			current = []string{}
			continue
		}
		if inBlock {
			current = append(current, line)
		}
	}
	if len(blocks) == 0 {
		return ""
	}
	if last {
		return blocks[len(blocks)-1]
	}
	return blocks[0]
}
