---
description: Example skill bundled with the exgen plugin. Replace this description with a specific trigger condition — Claude reads it to decide when to use the skill automatically (e.g. "Use when the user asks to generate release notes from recent commits").
---

# Example Skill

This is a starter skill for the `exgen` plugin, invoked automatically by Claude when relevant, or manually as `/exgen:example-skill`.

Replace everything below with the instructions Claude should follow when this skill fires.

## Template checklist

- [ ] Rename this folder to something specific, e.g. `skills/release-notes/`
- [ ] Write a precise `description` with concrete trigger phrases — this is the only thing Claude sees before deciding to load the skill
- [ ] Add supporting scripts or reference files in this folder if the skill needs them
- [ ] Add `disable-model-invocation: true` to the frontmatter if this skill should only ever run when a user explicitly types `/exgen:<skill-name>`, never automatically
- [ ] Add `allowed-tools: Read, Grep` (or similar) to the frontmatter to restrict what this skill can do
