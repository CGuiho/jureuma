// Package welcome renders the greeting printed by a bare `jureuma` invocation.
//
// The layout follows the GUIHO RunX welcome (logo, tagline, author line, then a
// platform/version/author block and a help hint), left-aligned instead of
// centered.
package welcome

import (
	"os"
	"runtime"
	"strings"
)

const (
	ansiReset = "\x1b[0m"
	ansiBold  = "\x1b[1m"
	ansiDim   = "\x1b[2m"

	// GUIHO palette: 8D99AE, EDF2F4, with blue 3B82F6 as the accent.
	ansiSlate    = "\x1b[38;2;141;153;174m"
	ansiOffWhite = "\x1b[38;2;237;242;244m"
	ansiBlue     = "\x1b[38;2;59;130;246m"
)

const (
	Tagline = "Agent-safe configuration"
	Author  = "GUIHO · Cristóvão GUIHO"
	Website = "www.guiho.co"
)

// ANSI Shadow glyphs, six rows each.
var glyphs = map[rune][6]string{
	'J': {
		"     ██╗",
		"     ██║",
		"     ██║",
		"██   ██║",
		"╚█████╔╝",
		" ╚════╝ ",
	},
	'U': {
		"██╗   ██╗",
		"██║   ██║",
		"██║   ██║",
		"██║   ██║",
		"╚██████╔╝",
		" ╚═════╝ ",
	},
	'R': {
		"██████╗ ",
		"██╔══██╗",
		"██████╔╝",
		"██╔══██╗",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
	'E': {
		"███████╗",
		"██╔════╝",
		"█████╗  ",
		"██╔══╝  ",
		"███████╗",
		"╚══════╝",
	},
	'M': {
		"███╗   ███╗",
		"████╗ ████║",
		"██╔████╔██║",
		"██║╚██╔╝██║",
		"██║ ╚═╝ ██║",
		"╚═╝     ╚═╝",
	},
	'A': {
		" █████╗ ",
		"██╔══██╗",
		"███████║",
		"██╔══██║",
		"██║  ██║",
		"╚═╝  ╚═╝",
	},
}

// Logo returns the JUREUMA block-letter logo, one string per row, with
// trailing spaces trimmed.
func Logo() []string {
	rows := make([]string, 6)
	for _, r := range "JUREUMA" {
		g := glyphs[r]
		for i := range rows {
			rows[i] += g[i]
		}
	}
	for i := range rows {
		rows[i] = strings.TrimRight(rows[i], " ")
	}
	return rows
}

var platformLabels = map[string]string{
	"darwin":  "macOS",
	"linux":   "Linux",
	"windows": "Windows",
}

var archLabels = map[string]string{
	"amd64": "x64",
	"386":   "x86",
	"arm64": "arm64",
	"arm":   "arm",
}

func label(m map[string]string, k string) string {
	if v, ok := m[k]; ok {
		return v
	}
	return k
}

// Render returns the welcome text. Empty goos/goarch default to the running
// platform. color adds the GUIHO ANSI palette.
func Render(goos, goarch, version string, color bool) string {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	paint := func(s, code string) string {
		if !color {
			return s
		}
		return code + s + ansiReset
	}

	var b strings.Builder
	// Blank lines frame the page so it never touches the prompt lines.
	b.WriteByte('\n')
	for _, row := range Logo() {
		b.WriteString(paint(row, ansiBlue+ansiBold))
		b.WriteByte('\n')
	}
	b.WriteString(paint(Tagline, ansiSlate+ansiDim))
	b.WriteByte('\n')
	b.WriteString(paint(Author, ansiSlate))
	b.WriteString("\n\n")

	b.WriteString(paint("Platform", ansiSlate))
	b.WriteString("  ")
	b.WriteString(paint(label(platformLabels, goos)+" "+label(archLabels, goarch), ansiOffWhite))
	b.WriteByte('\n')
	b.WriteString(paint("Version", ansiSlate))
	b.WriteString("   ")
	b.WriteString(paint("v"+strings.TrimPrefix(version, "v"), ansiBlue+ansiBold))
	b.WriteByte('\n')
	b.WriteString(paint("Author", ansiSlate))
	b.WriteString("    ")
	b.WriteString(paint("GUIHO (", ansiOffWhite))
	b.WriteString(paint(Website, ansiBlue))
	b.WriteString(paint(")", ansiOffWhite))
	b.WriteString("\n\n")

	b.WriteString("Run ")
	b.WriteString(paint("jureuma --help", ansiOffWhite+ansiBold))
	b.WriteString(" to see available commands.\n\n")
	return b.String()
}

// UseColor reports whether to colour output going to a terminal.
func UseColor(isTerminal bool) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return isTerminal
}
