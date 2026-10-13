---
subject: jureuma-agentres
description: "Embedded agent artifacts and their installers: the guiho-s-jureuma skill, the prompts (guiho-p-jureuma, guiho-p-jureuma-install, guiho-p-jureuma-uninstall) and the AGENTS.md instruction block, all embedded from assets/ (assets/skills/guiho-s-jureuma/SKILL.md, assets/prompts/*.md, assets/instruction.md). Descriptors inside assets/ (*.xdocs.md) are never shipped: enumeration and release packaging skip them."
parent: jureuma-internal
children:
  - jureuma-agentres-assets
files:
  "agentres.go": "Embeds assets/ and exposes skills, prompts and the instruction body."
  "block.go": "Managed AGENTS.md block markers and apply/remove helpers."
  "install.go": "Skill destinations (~/.claude/skills, ~/.agents/skills, ~/.codex/skills) and skill installation."
  "agentres_test.go": "Tests for embedded artifacts, block handling and skill installation."
documents: {}
tags:
  - "go"
  - "agents"
keywords:
  - "skill"
  - "prompt"
  - "AGENTS.md"
  - "guiho-s-jureuma"
flags: []
status: stable
---

# jureuma-agentres

Embedded agent artifacts and their installers: the guiho-s-jureuma skill, the prompts (guiho-p-jureuma, guiho-p-jureuma-install, guiho-p-jureuma-uninstall) and the AGENTS.md instruction block, all embedded from assets/ (assets/skills/guiho-s-jureuma/SKILL.md, assets/prompts/*.md, assets/instruction.md). Descriptors inside assets/ (*.xdocs.md) are never shipped: enumeration and release packaging skip them.

## Files

- `agentres.go`: Embeds assets/ and exposes skills, prompts and the instruction body.
- `block.go`: Managed AGENTS.md block markers and apply/remove helpers.
- `install.go`: Skill destinations (~/.claude/skills, ~/.agents/skills, ~/.codex/skills) and skill installation.
- `agentres_test.go`: Tests for embedded artifacts, block handling and skill installation.
