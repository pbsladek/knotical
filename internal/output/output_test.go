package output

import (
	"strings"
	"testing"
)

func TestRenderMarkdownStripsFencesAndStylesHeadings(t *testing.T) {
	input := "# Title\n\nParagraph with `code`.\n\n```go\nfmt.Println(\"hi\")\n```"
	rendered := RenderMarkdown(input)

	if strings.Contains(rendered, "```") {
		t.Fatalf("expected code fences to be removed: %q", rendered)
	}
	if !strings.Contains(rendered, "Title") {
		t.Fatalf("expected heading text in rendered output: %q", rendered)
	}
	if !strings.Contains(rendered, "fmt.Println(\"hi\")") {
		t.Fatalf("expected code block content in rendered output: %q", rendered)
	}
}

func TestRenderMarkdownHandlesListsAndQuotes(t *testing.T) {
	input := "> note\n- first\n1. second"
	rendered := RenderMarkdown(input)

	if !strings.Contains(rendered, "> note") {
		t.Fatalf("expected rendered blockquote, got %q", rendered)
	}
	if !strings.Contains(rendered, "- first") {
		t.Fatalf("expected rendered bullet list, got %q", rendered)
	}
	if !strings.Contains(rendered, "1. second") {
		t.Fatalf("expected rendered numbered list, got %q", rendered)
	}
}

func TestPrinterMethodsAndDefaultPrinter(t *testing.T) {
	var buffer strings.Builder
	printer := NewPrinter(&buffer)

	printer.Header("header")
	printer.Success("ok")
	printer.Warn("warn")
	printer.ListItem("name", "detail")
	printer.ListItem("plain", "")
	printer.Print("raw")
	printer.Println(" line")
	printer.Prompt("> ")
	printer.PrintResponse("text", false)
	printer.PrintResponse("# Heading", true)

	output := buffer.String()
	for _, want := range []string{"header", "ok", "warn", "name\tdetail", "plain", "raw", "> ", "text", "Heading"} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected output to contain %q, got %q", want, output)
		}
	}

	var defaultBuffer strings.Builder
	restore := SetDefaultPrinter(NewPrinter(&defaultBuffer))
	defer restore()

	Header("h")
	Success("s")
	Warn("w")
	ListItem("item", "detail")
	Print("x")
	Println("y")
	Prompt("$ ")
	PrintResponse("z", false)

	defaultOutput := defaultBuffer.String()
	for _, want := range []string{"h", "s", "w", "item\tdetail", "xy", "$ ", "z"} {
		if !strings.Contains(defaultOutput, want) {
			t.Fatalf("expected default printer output to contain %q, got %q", want, defaultOutput)
		}
	}
}

func TestPrinterSanitizesTerminalControlSequences(t *testing.T) {
	var buffer strings.Builder
	printer := NewPrinter(&buffer)

	printer.Println("hello\x1b[2Jworld\r\nnext")
	printer.ListItem("na\x07me", "de\x1b]0;title\x07tail")
	printer.PrintResponse("body\x1b[31mred", false)

	got := buffer.String()
	if strings.Contains(got, "\x1b") || strings.Contains(got, "\r") || strings.Contains(got, "\x07") {
		t.Fatalf("expected control sequences to be removed, got %q", got)
	}
	if !strings.Contains(got, "helloworld") || !strings.Contains(got, "name\tdetail") || !strings.Contains(got, "bodyred") {
		t.Fatalf("unexpected sanitized output: %q", got)
	}
}
