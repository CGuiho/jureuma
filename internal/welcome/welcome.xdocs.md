---
subject: jureuma-welcome
description: "Left-aligned welcome page printed by a bare jureuma invocation: JUREUMA ANSI Shadow logo, tagline, author, creator/platform/version block and help hint, in the GUIHO palette when stdout is a terminal."
parent: jureuma-internal
children: []
files:
  "welcome.go": "Logo glyphs, Render and colour detection (NO_COLOR, TERM=dumb)."
  "welcome_test.go": "Tests for glyph alignment, plain and coloured output, and left alignment."
documents: {}
tags:
  - "go"
  - "cli"
keywords:
  - "welcome"
  - "banner"
flags: []
status: stable
---

# jureuma-welcome

Left-aligned welcome page printed by a bare `jureuma` invocation, modelled on the RunX welcome but not centered: JUREUMA logo, tagline, author, creator/platform/version block and help hint. Colour uses the GUIHO palette only when stdout is a terminal and `NO_COLOR` is unset.

## Files

- `welcome.go`: Logo glyphs, Render and colour detection (NO_COLOR, TERM=dumb).
- `welcome_test.go`: Tests for glyph alignment, plain and coloured output, and left alignment.
