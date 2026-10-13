package welcome

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestLogoRowsAligned(t *testing.T) {
	rows := Logo()
	if len(rows) != 6 {
		t.Fatalf("got %d rows", len(rows))
	}
	for i, g := range glyphs {
		w := utf8.RuneCountInString(g[0])
		for _, row := range g {
			if utf8.RuneCountInString(row) != w {
				t.Fatalf("glyph %q has rows of different widths", i)
			}
		}
	}
}

func TestRenderPlain(t *testing.T) {
	out := Render("linux", "amd64", "v0.1.0", false)
	if strings.Contains(out, "\x1b[") {
		t.Fatal("plain output contains ANSI escapes")
	}
	for _, want := range []string{
		Logo()[0] + "\n",
		"\n" + Tagline + "\n" + Author + "\n\n\n",
		"Creator    Cristóvão GUIHO\n",
		"  platform      Linux x64\n",
		"  version       v0.1.0\n\n",
		"Run jureuma --help to see available commands.\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n%s", want, out)
		}
	}
	// Left-aligned: no line starts with centering spaces except the
	// indented detail rows and the J glyph's own leading spaces.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, Tagline) || strings.HasPrefix(line, Author) {
			continue
		}
		if strings.HasPrefix(line, "   ") && !strings.HasPrefix(line, "     ██╗") && !strings.HasPrefix(line, "     ██║") {
			t.Errorf("unexpected indentation: %q", line)
		}
	}
}

func TestRenderColor(t *testing.T) {
	out := Render("darwin", "arm64", "1.2.3", true)
	if !strings.Contains(out, ansiBrightRed) || !strings.Contains(out, "macOS arm64") || !strings.Contains(out, "v1.2.3") {
		t.Fatalf("unexpected colour output:\n%s", out)
	}
}

func TestUseColor(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("TERM", "xterm-256color")
	if !UseColor(true) || UseColor(false) {
		t.Fatal("colour should follow the terminal")
	}
	t.Setenv("NO_COLOR", "1")
	if UseColor(true) {
		t.Fatal("NO_COLOR must disable colour")
	}
}
