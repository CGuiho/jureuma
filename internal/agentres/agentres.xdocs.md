---
subject: zeze-agentres
description: "Embedded agent artifacts and their installers: the guiho-s-zeze skill, the prompts (guiho-p-zeze, guiho-p-zeze-install, guiho-p-zeze-uninstall) and the AGENTS.md instruction block, all embedded from assets/ (assets/skills/guiho-s-zeze/SKILL.md, assets/prompts/*.md, assets/instruction.md). Descriptors inside assets/ (*.xdocs.md) are never shipped: enumeration and release packaging skip them."
parent: zeze-internal
children:
  - zeze-agentres-assets
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
  - "guiho-s-zeze"
flags: []
status: stable
---

# zeze-agentres

Embedded agent artifacts and their installers: the guiho-s-zeze skill, the prompts (guiho-p-zeze, guiho-p-zeze-install, guiho-p-zeze-uninstall) and the AGENTS.md instruction block, all embedded from assets/ (assets/skills/guiho-s-zeze/SKILL.md, assets/prompts/*.md, assets/instruction.md). Descriptors inside assets/ (*.xdocs.md) are never shipped: enumeration and release packaging skip them.

## Files

- `agentres.go`: Embeds assets/ and exposes skills, prompts and the instruction body.
- `block.go`: Managed AGENTS.md block markers and apply/remove helpers.
- `install.go`: Skill destinations (~/.claude/skills, ~/.agents/skills, ~/.codex/skills) and skill installation.
- `agentres_test.go`: Tests for embedded artifacts, block handling and skill installation.
