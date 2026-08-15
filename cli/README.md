# exgen CLI

Install a Claude Code plugin's skills, agents and commands into whichever
coding tool you actually use, translating each component into that tool's
on-disk conventions and reporting anything that could not cross over intact.

```bash
go install github.com/trungleque/exgen-plugin/cli/cmd/exgen@latest
```

The exgen plugin is compiled into the binary, so the common case needs no
checkout:

```bash
exgen install pi          # install the exgen toolkit into Pi
exgen install all         # ...into every supported tool
```

Point it at any other Claude Code plugin with `--from`:

```bash
exgen install codex --from ../some-other-plugin
```

## Commands

| Command | What it does |
| :--- | :--- |
| `exgen install <target>...` | Translate and write the plugin's components |
| `exgen uninstall <target>...` | Remove what a previous install wrote |
| `exgen list [target]...` | Show the plugin's components, and how a target handles each kind |
| `exgen targets` | Show every supported tool and where it writes |

Common flags: `--from <dir>`, `--project` (default is `--global`), `--dry-run`,
`--only skills,agents,commands`.

`all` works anywhere a target is accepted. Targets also accept aliases —
`chatgpt` and `openai` mean `codex`, `cc` and `claude-code` mean `claude`,
`ag` means `antigravity`.

## Targets

Claude Code's plugin layout is the source format, so a `claude` install is a
byte-for-byte copy. Everything else is a translation.

| | Claude Code | Pi | Antigravity CLI | Codex / ChatGPT |
| :--- | :--- | :--- | :--- | :--- |
| **Skills** | `~/.claude/skills/<n>/SKILL.md` | `~/.pi/agent/skills/<n>/SKILL.md` | plugin `skills/<n>/SKILL.md` | `~/.agents/skills/<n>/SKILL.md` |
| **Agents** | `~/.claude/agents/<n>.md` | `~/.pi/agent/agents/<n>.md` † | plugin `agents/<n>.md` | `~/.codex/agents/<n>.toml` |
| **Commands** | `~/.claude/commands/<n>.md` | `~/.pi/agent/prompts/<n>.md` | → skill | → skill |

† Requires the [pi-subagents](https://pi.dev/packages/pi-subagents) package;
the install prints the command.

With `--project`, each tool writes its repository-local equivalent —
`.claude/`, `.pi/`, `.agents/` and `.codex/`. Note that Codex, Pi and
Antigravity all read project skills from `.agents/`, so installing several of
them at project scope populates one shared directory.

Run `exgen targets` for the resolved paths on your machine.

## What translation actually changes

Every one of these is reported on the line it affects, so an install is never
silently lossy.

- **Codex agents become TOML.** Codex defines a subagent as a TOML file with
  the system prompt in a `developer_instructions` key, not as markdown with
  frontmatter. The body is carried across verbatim.
- **Commands become skills** on Codex, which has no slash commands, and on
  Antigravity, which exposes skills *as* slash commands. The description gains
  a trigger hint, because a skill is selected by its description where a
  command was selected by name.
- **`$ARGUMENTS` becomes `$@`** in Pi prompt templates. Positional `$1`…`$9`
  already mean the same thing in both.
- **Tool allowlists are remapped or dropped.** Claude's `Glob, Grep, LS, Read`
  becomes Pi's `find, grep, ls, read`; names with no equivalent are dropped and
  named in the report. Where a tool's naming is not documented, the allowlist
  is removed entirely rather than replaced with names that would match nothing
  and leave the agent unable to act.
- **`model: sonnet` is dropped** everywhere but Claude Code. It is a
  Claude-only alias, so leaving it in place stops the agent from starting
  rather than making it fall back.
- **Unknown frontmatter keys are stripped** to each tool's documented schema.
- **`disable-model-invocation` has no equivalent** outside Claude Code and Pi.
  On Pi, a skill hidden from the model would have no invocation path left, so a
  prompt template is installed alongside it as the human entry point.
- **Relative links are rewritten** when a layout moves a skill file relative to
  its bundled assets — Antigravity's flat project layout is the case that needs
  it.

## Uninstall

`uninstall` removes what an install recorded, and refuses to delete a file that
has changed since it was written:

```
- structured-prompt   ~/.claude/skills/structured-prompt/SKILL.md
! review              ~/.claude/skills/review/SKILL.md

10 removed, 1 locally modified, kept
```

`--force` removes them anyway.

The record lives in `~/.exgen/manifest.json` (or `.exgen/manifest.json` for a
project install) and stores a digest per file. Comparing against a digest taken
at install time, rather than against what the current binary would write, is
what keeps a version upgrade from being misread as a local edit. The file is
deleted once nothing is tracked.

## Development

```bash
make build     # sync the embedded plugin, then build ./exgen
make check     # go vet + go test ./...
make test
```

`go:embed` cannot reach outside its own module, and the module root is `cli/`
while the plugin lives at `../plugins/exgen`. `make sync` therefore mirrors the
plugin into `internal/bundled/exgen/`, which is committed so that `go install`
works from a bare checkout. `TestBundledMatchesSource` fails if the mirror and
the real plugin drift, so editing a skill without re-syncing breaks the build
rather than shipping stale instructions.

### Package layout

| Package | Responsibility |
| :--- | :--- |
| `cmd/exgen` | The binary. It lives a directory down so that `go install` names it `exgen` rather than `cli`, after the module path's last element |
| `internal/plugin` | Parse a Claude Code plugin into a target-neutral model; ordered frontmatter that survives a round trip |
| `internal/target` | One file per tool, each answering: where do files go, which kinds exist natively, what is lost |
| `internal/install` | Plan / apply / remove, plus the install manifest |
| `internal/bundled` | The embedded plugin and its drift guard |
| `internal/cmd` | cobra wiring and report rendering |

Adding a target means implementing `target.Target` and calling `register()` in
an `init`. `TestSupportTableMatchesPlan` then enforces that its advertised
capabilities match what it actually plans.
