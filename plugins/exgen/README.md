# exgen

A Claude Code plugin for spec-driven development: turn a feature or
requirement into small, human-reviewable **structured prompts** (simplified
SPDD / REASONS-style canvases), implement them with strict TDD, and gate every
task behind an independent review. The review surface is a handful of short
markdown files — checked *before* code is generated, and verified against
again after.

## Components

### Skills

| Skill | Invocation | What it does |
| :--- | :--- | :--- |
| `structured-prompt` | Auto-invoked, or `/exgen:structured-prompt` | Generates **one** reviewable structured prompt from a feature description |
| `user-story` | `/exgen:user-story [feature description]` (explicit only) | Full planning workflow: explores the codebase, asks clarifying questions, designs architecture options, decomposes into tasks, and emits one structured prompt per task |
| `implementation` | Auto-invoked, or `/exgen:implementation [prompt id]` | Implements a structured prompt with strict TDD — red-green-refactor, one Operation at a time |
| `review` | Auto-invoked, or `/exgen:review [prompt id]` | Independently verifies an implementation against its prompt; the gate that moves a prompt to `spdd/done/` on APPROVE |

**`structured-prompt`** produces a single markdown file (40–120 lines) with
five sections — Requirements (Scope In/Out), Entities, Approach, Operations,
Safeguards — short enough that a human can verify the whole plan in a couple
of minutes. It scans the codebase for the entities and patterns the feature
touches (or reuses an analysis already in the conversation) so the prompt is
grounded in what actually exists. Use it directly for a single small feature.

**`user-story`** is the end-to-end planning workflow for anything bigger. It
runs six phases: discovery → codebase exploration (via `code-explorer`
agents) → clarifying questions → architecture design (via `code-architect`
agents) → decomposition into the smallest independently checkable tasks → one
structured prompt per task (via the `structured-prompt` skill). It is
deliberately interactive — it stops to ask questions and to confirm the
architecture and task list before generating anything. It never auto-triggers
(`disable-model-invocation: true`); invoke it explicitly.

**`implementation`** executes a structured prompt as working, tested code.
The prompt is the contract: Scope In is the definition of done, Scope Out is
a hard boundary, Operations set the execution order, and every Safeguard must
be enforced by a test. Development is strict TDD — no production code without
a failing test first — guided by a bundled testing anti-patterns reference
(`skills/implementation/references/testing-anti-patterns.md`). If the codebase
contradicts the prompt, it stops and asks rather than silently improvising,
and it always ends by handing off to the `review` skill.

**`review`** is the mandatory quality gate. It re-runs the tests, audits the
diff section by section against the prompt (with per-item ✅/❌/⚠️ verdicts and
evidence), spot-checks critical Safeguard tests by mutation (break the code,
confirm the test fails, restore), and screens tests for anti-patterns. Verdict
is **APPROVE** — the only action in the pipeline that moves the prompt file
from `spdd/prompt/` to `spdd/done/` — or **REQUEST CHANGES**, which sends the
blocking findings back to the `implementation` skill. If the implementation
happened in the same conversation, it delegates the review to a subagent for
independence.

### Agents

| Agent | Model | What it does |
| :--- | :--- | :--- |
| `code-explorer` | sonnet | Read-only codebase analyst: traces a feature from entry points to data storage, maps architecture layers and patterns, and returns the key files to read |
| `code-architect` | sonnet | Read-only architect: extracts existing patterns and conventions, then delivers a decisive implementation blueprint — files to create/modify, component design, data flow, build sequence |

Both agents are used by the `user-story` workflow (exploration in phase 2,
architecture in phase 4), and Claude can also delegate to them directly for
standalone analysis or design questions.

## The pipeline

```
user-story / structured-prompt          implementation                review
   (plan)                                 (build, TDD)               (verify)
      │                                        │                        │
      └──> spdd/prompt/NNN-feature.md ──> tested code ──> APPROVE ──> spdd/done/
                    ▲                                        │
                    └──────────── REQUEST CHANGES <──────────┘
```

`spdd/prompt/` is the queue of work awaiting implementation or review;
`spdd/done/` holds approved, verified tasks. "Implement the next prompt"
always means the lowest-numbered file still in `spdd/prompt/`.

## Output

Generated prompts are saved under `spdd/prompt/` in the project being planned,
named `[id]-[kebab-case-feature-name].md` — where `[id]` is the ticket id if
one was given (e.g. `JIRA-123-...`), otherwise a `001`-style sequence number.
Multi-task stories under one ticket add a task index (`JIRA-123-1-...`,
`JIRA-123-2-...`). Once a task passes review, its prompt file moves to
`spdd/done/` with the same name.

## Add more components

- **More skills**: add a new folder under `skills/`, each with its own `SKILL.md`
- **Commands**: add `.md` files under `commands/`
- **More agents**: add a new `.md` file under `agents/`
- **Hooks / MCP / LSP**: add `hooks/hooks.json`, `.mcp.json`, or `.lsp.json` at the plugin root

See the [Plugins reference](https://code.claude.com/docs/en/plugins-reference) for full schemas.

## License

[MIT](../../LICENSE) — free for everyone to use, modify, and share.
