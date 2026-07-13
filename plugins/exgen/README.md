# exgen

A Claude Code plugin for spec-driven planning: turn a feature or requirement
into small, human-reviewable **structured prompts** (simplified SPDD /
REASONS-style canvases) *before* any code is generated. The review surface is
a handful of short markdown files, not a wall of generated code.

## Components

### Skills

| Skill | Invocation | What it does |
| :--- | :--- | :--- |
| `structured-prompt` | Auto-invoked, or `/exgen:structured-prompt` | Generates **one** reviewable structured prompt from a feature description |
| `user-story` | `/exgen:user-story [feature description]` (explicit only) | Full planning workflow: explores the codebase, asks clarifying questions, designs architecture options, decomposes into tasks, and emits one structured prompt per task |

**`structured-prompt`** produces a single markdown file (40–120 lines) with
five sections — Requirements (Scope In/Out), Entities, Approach, Operations,
Safeguards — short enough that a human can verify the whole plan in a couple
of minutes. It scans the codebase for the entities and patterns the feature
touches (or reuses an analysis already in the conversation) so the prompt is
grounded in what actually exists. Use it directly for a single small feature.

**`user-story`** is the end-to-end workflow for anything bigger. It runs six
phases: discovery → codebase exploration (via `code-explorer` agents) →
clarifying questions → architecture design (via `code-architect` agents) →
decomposition into the smallest independently checkable tasks → one structured
prompt per task (via the `structured-prompt` skill). It is deliberately
interactive — it stops to ask questions and to confirm the architecture and
task list before generating anything. It never auto-triggers
(`disable-model-invocation: true`); invoke it explicitly.

### Agents

| Agent | Model | What it does |
| :--- | :--- | :--- |
| `code-explorer` | sonnet | Read-only codebase analyst: traces a feature from entry points to data storage, maps architecture layers and patterns, and returns the key files to read |
| `code-architect` | sonnet | Read-only architect: extracts existing patterns and conventions, then delivers a decisive implementation blueprint — files to create/modify, component design, data flow, build sequence |

Both agents are used by the `user-story` workflow (exploration in phase 2,
architecture in phase 4), and Claude can also delegate to them directly for
standalone analysis or design questions.

## Output

Generated prompts are saved under `spdd/prompt/` in the project being planned,
named `[id]-[kebab-case-feature-name].md` — where `[id]` is the ticket id if
one was given (e.g. `JIRA-123-...`), otherwise a `001`-style sequence number.
Multi-task stories under one ticket add a task index (`JIRA-123-1-...`,
`JIRA-123-2-...`).

## Add more components

- **More skills**: add a new folder under `skills/`, each with its own `SKILL.md`
- **Commands**: add `.md` files under `commands/`
- **More agents**: add a new `.md` file under `agents/`
- **Hooks / MCP / LSP**: add `hooks/hooks.json`, `.mcp.json`, or `.lsp.json` at the plugin root

See the [Plugins reference](https://code.claude.com/docs/en/plugins-reference) for full schemas.

## License

[MIT](../../LICENSE) — free for everyone to use, modify, and share.
