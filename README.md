# exgen

A spec-driven development toolkit that turns feature requirements into small,
human-reviewable structured prompts, implements them with strict TDD, and gates
every task behind an independent review.

It ships as a **CLI** that installs the toolkit into whichever coding tool you
already use — Claude Code, [Pi](https://pi.dev),
[Antigravity](https://antigravity.google), or Codex / ChatGPT — translating
every component into that tool's own on-disk conventions.

```bash
exgen install claude       # or: pi, antigravity, codex, all
```

## Install the CLI

### With `go install`

Requires Go 1.24 or newer.

```bash
go install github.com/trungleque/exgen-plugin/cli/cmd/exgen@latest
```

The toolkit itself is compiled into the binary, so nothing else needs cloning.
Make sure `$(go env GOPATH)/bin` is on your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### From source

```bash
git clone https://github.com/trungleque/exgen-plugin.git
cd exgen-plugin/cli
make build          # produces ./exgen
```

`make install` puts it on your `GOPATH/bin` instead.

### Check it worked

```bash
exgen --version
exgen list          # the skills and agents that will be installed
```

## Install the toolkit into your tool

Preview first — nothing is written:

```bash
exgen install all --dry-run
```

Then install for real:

```bash
exgen install claude          # user-level, the default
exgen install pi codex        # several tools at once
exgen install antigravity --project   # into the current repository instead
```

`exgen targets` prints every supported tool and the exact paths it writes on
your machine. To back it out, `exgen uninstall <tool>` — which refuses to
delete any file you have edited since it was installed.

| | Skills | Agents | Commands |
| :--- | :--- | :--- | :--- |
| **Claude Code** | `~/.claude/skills/` | `~/.claude/agents/` | `~/.claude/commands/` |
| **Pi** | `~/.pi/agent/skills/` | `~/.pi/agent/agents/` † | `~/.pi/agent/prompts/` |
| **Antigravity** | plugin `skills/` | plugin `agents/` | → skill |
| **Codex / ChatGPT** | `~/.agents/skills/` | `~/.codex/agents/*.toml` | → skill |

† Needs the [pi-subagents](https://pi.dev/packages/pi-subagents) package; the
install prints the command.

Claude Code's plugin layout is the source format, so installing there is a
byte-for-byte copy. Every other tool is a translation, and anything that cannot
cross the boundary intact — Codex has no slash commands, Pi has no built-in
subagents, `model: sonnet` resolves nowhere but Claude Code — is reported per
component rather than silently reinterpreted. See
[`cli/README.md`](cli/README.md) for the full list of what translation changes.

## What gets installed

Four skills and two agents:

- **`structured-prompt`** (skill) — generate one concise, reviewable
  structured prompt (a simplified SPDD / REASONS canvas) from a feature
  description
- **`user-story`** (skill) — the full planning workflow: codebase exploration,
  clarifying questions, architecture options, task decomposition, then one
  structured prompt per task
- **`implementation`** (skill) — execute a structured prompt as tested code
  using strict red-green-refactor TDD, one Operation at a time
- **`review`** (skill) — independently verify an implementation against its
  prompt; the mandatory gate that moves a prompt to `spdd/done/` on APPROVE
- **`code-explorer`** (agent) — traces how existing features work, from entry
  points to data storage
- **`code-architect`** (agent) — designs implementation blueprints grounded in
  the codebase's existing patterns

See [`plugins/exgen/README.md`](plugins/exgen/README.md) for component details.

In Claude Code the skills are then invocable as `/exgen:<skill-name>`:

```
/exgen:user-story Add rate limiting to the password reset endpoint
```

## Installing any other plugin

The CLI is a general translator, not an exgen-specific installer. Point it at
any Claude Code plugin directory:

```bash
exgen install pi --from ../some-other-plugin
```

## Repository layout

```
.
├── LICENSE                          # MIT
├── README.md
├── cli/                             # the Go CLI
│   ├── Makefile                     # `make build` syncs the embedded plugin, then builds
│   ├── cmd/exgen/main.go            # the binary
│   └── internal/
│       ├── plugin/                  # parse a Claude Code plugin
│       ├── target/                  # one file per tool: claude, pi, antigravity, codex
│       ├── install/                 # plan / apply / remove + install manifest
│       ├── bundled/                 # embedded copy of plugins/exgen (see cli/README.md)
│       └── cmd/                     # cobra wiring
└── plugins/
    └── exgen/                       # the toolkit — the CLI's source format
        ├── .claude-plugin/
        │   └── plugin.json              # plugin manifest (name, version, author)
        ├── skills/                      # -> /exgen:<skill-name>
        │   ├── structured-prompt/
        │   │   └── SKILL.md             # one feature -> one reviewable prompt
        │   ├── user-story/
        │   │   └── SKILL.md             # full story workflow -> prompt per task
        │   ├── implementation/
        │   │   ├── SKILL.md             # prompt -> tested code, strict TDD
        │   │   └── references/
        │   │       └── testing-anti-patterns.md
        │   └── review/
        │       └── SKILL.md             # verify code against prompt, gate to spdd/done/
        ├── agents/                      # subagents Claude can delegate to
        │   ├── code-explorer.md
        │   └── code-architect.md
        └── README.md
```

`plugins/exgen/.claude-plugin/` only ever contains the `plugin.json` manifest.
Every other directory (`skills/`, `agents/`, `commands/`, `hooks/`, etc.) lives
at the plugin root, one level up — a component placed inside `.claude-plugin/`
is invisible.

## Develop

Work on the toolkit by loading it straight from disk, with no install:

```bash
claude --plugin-dir ./plugins/exgen
```

`/reload-plugins` picks up edits without restarting the session.

Validate before sharing:

```bash
claude plugin validate ./plugins/exgen   # checks the plugin manifest + components
```

Work on the CLI from `cli/`:

```bash
make check          # go vet + go test ./...
make build
```

Editing a skill means re-running `make sync` (or `make build`, which does it)
so the copy embedded in the binary keeps up — a test fails if the two drift.

## License

[MIT](LICENSE) — free for everyone to use, modify, and share.

Reference: [Create plugins](https://code.claude.com/docs/en/plugins) ·
[Plugins reference](https://code.claude.com/docs/en/plugins-reference)
