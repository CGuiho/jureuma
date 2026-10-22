---
subject: zeze-welcome
description: "Left-aligned welcome page printed by a bare zeze invocation: ZEZE ANSI Shadow logo, tagline, author, platform/version/author block and help hint, in blue and the GUIHO palette when stdout is a terminal."
parent: zeze-internal
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

# zeze-welcome

Left-aligned welcome page printed by a bare `zeze` invocation, modelled on the RunX welcome but not centered: ZEZE logo, tagline, author, platform/version/author block and help hint. Colour uses blue for the logo and version, with the GUIHO slate and off-white only when stdout is a terminal and `NO_COLOR` is unset.

## Files

- `welcome.go`: Logo glyphs, Render and colour detection (NO_COLOR, TERM=dumb).
- `welcome_test.go`: Tests for glyph alignment, plain and coloured output, and left alignment.
