package output

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	green  = "\033[32m"
	yellow = "\033[33m"
)

type Printer struct {
	Out io.Writer
}

func NewPrinter(out io.Writer) *Printer {
	if out == nil {
		out = io.Discard
	}
	return &Printer{Out: out}
}

func (p *Printer) Header(text string) {
	fmt.Fprintf(p.Out, "%s%s%s%s\n", bold, cyan, sanitizeTerminalText(text), reset)
}

func (p *Printer) Success(text string) {
	fmt.Fprintf(p.Out, "%s%s%s\n", green, sanitizeTerminalText(text), reset)
}

func (p *Printer) Warn(text string) {
	fmt.Fprintf(p.Out, "%s%s%s\n", yellow, sanitizeTerminalText(text), reset)
}

func (p *Printer) ListItem(name string, detail string) {
	name = sanitizeTerminalText(name)
	detail = sanitizeTerminalText(detail)
	if detail == "" {
		fmt.Fprintln(p.Out, name)
		return
	}
	fmt.Fprintf(p.Out, "%s\t%s\n", name, detail)
}

func (p *Printer) Print(text string) {
	fmt.Fprint(p.Out, sanitizeTerminalText(text))
}

func (p *Printer) Println(text string) {
	fmt.Fprintln(p.Out, sanitizeTerminalText(text))
}

func (p *Printer) Prompt(prefix string) {
	fmt.Fprint(p.Out, sanitizeTerminalText(prefix))
}

var (
	inlineCodePattern = regexp.MustCompile("`([^`]+)`")
	boldPattern       = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	italicPattern     = regexp.MustCompile(`\*([^*\n]+)\*`)
	numberedPattern   = regexp.MustCompile(`^\d+\.\s+`)
	csiPattern        = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)
	oscPattern        = regexp.MustCompile(`\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)`)
)

func (p *Printer) PrintResponse(text string, markdown bool) {
	text = sanitizeTerminalText(text)
	if markdown {
		fmt.Fprint(p.Out, RenderMarkdown(text))
		if !strings.HasSuffix(text, "\n") {
			fmt.Fprint(p.Out, "\n")
		}
		return
	}
	p.Println(text)
}

var defaultPrinter = NewPrinter(os.Stdout)

func SetDefaultPrinter(printer *Printer) func() {
	if printer == nil {
		printer = NewPrinter(io.Discard)
	}
	previous := defaultPrinter
	defaultPrinter = printer
	return func() {
		defaultPrinter = previous
	}
}

func Header(text string) { defaultPrinter.Header(text) }

func Success(text string) { defaultPrinter.Success(text) }

func Warn(text string) { defaultPrinter.Warn(text) }

func ListItem(name string, detail string) { defaultPrinter.ListItem(name, detail) }

func Print(text string) { defaultPrinter.Print(text) }

func Println(text string) { defaultPrinter.Println(text) }

func Prompt(prefix string) { defaultPrinter.Prompt(prefix) }

func PrintResponse(text string, markdown bool) { defaultPrinter.PrintResponse(text, markdown) }

func RenderMarkdown(text string) string {
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines))
	inCodeBlock := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			if !inCodeBlock {
				out = append(out, "")
			}
			continue
		}

		if inCodeBlock {
			out = append(out, fmt.Sprintf("%s    %s%s", green, line, reset))
			continue
		}

		switch {
		case trimmed == "":
			out = append(out, "")
		case strings.HasPrefix(trimmed, "# "):
			out = append(out, fmt.Sprintf("%s%s%s%s", bold, cyan, renderInlineMarkdown(strings.TrimSpace(trimmed[2:])), reset))
		case strings.HasPrefix(trimmed, "## "):
			out = append(out, fmt.Sprintf("%s%s%s%s", bold, cyan, renderInlineMarkdown(strings.TrimSpace(trimmed[3:])), reset))
		case strings.HasPrefix(trimmed, "### "):
			out = append(out, fmt.Sprintf("%s%s%s%s", bold, cyan, renderInlineMarkdown(strings.TrimSpace(trimmed[4:])), reset))
		case strings.HasPrefix(trimmed, "> "):
			out = append(out, fmt.Sprintf("%s> %s%s", yellow, renderInlineMarkdown(strings.TrimSpace(trimmed[2:])), reset))
		case strings.HasPrefix(trimmed, "- "), strings.HasPrefix(trimmed, "* "):
			out = append(out, fmt.Sprintf("- %s", renderInlineMarkdown(strings.TrimSpace(trimmed[2:]))))
		case numberedPattern.MatchString(trimmed):
			index := strings.Index(trimmed, ".")
			out = append(out, fmt.Sprintf("%s", renderInlineMarkdown(trimmed[:index+2]+strings.TrimSpace(trimmed[index+1:]))))
		default:
			out = append(out, renderInlineMarkdown(line))
		}
	}

	return strings.Join(out, "\n")
}

func renderInlineMarkdown(text string) string {
	text = inlineCodePattern.ReplaceAllString(text, cyan+"$1"+reset)
	text = boldPattern.ReplaceAllString(text, bold+"$1"+reset)
	text = italicPattern.ReplaceAllString(text, "$1")
	return text
}

func sanitizeTerminalText(text string) string {
	text = oscPattern.ReplaceAllString(text, "")
	text = csiPattern.ReplaceAllString(text, "")
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20 || r == 0x7f:
			return -1
		case r >= 0x80 && r <= 0x9f:
			return -1
		default:
			return r
		}
	}, text)
}
