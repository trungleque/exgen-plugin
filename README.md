# exgen

A Claude Code plugin repository containing the `exgen` plugin: a spec-driven
planning toolkit that turns feature requirements into small, human-reviewable
structured prompts before any code is generated.

The plugin bundles two skills and two agents:

- **`structured-prompt`** (skill) — generate one concise, reviewable
  structured prompt (a simplified SPDD / REASONS canvas) from a feature
  description
- **`user-story`** (skill) — the full planning workflow: codebase exploration,
  clarifying questions, architecture options, task decomposition, then one
  structured prompt per task
- **`code-explorer`** (agent) — traces how existing features work, from entry
  points to data storage
- **`code-architect`** (agent) — designs implementation blueprints grounded in
  the codebase's existing patterns

See [`plugins/exgen/README.md`](plugins/exgen/README.md) for component details.

## Repository layout

```
.
├── .claude-plugin/
│   └── marketplace.json             # marketplace catalog — lists the exgen plugin
├── LICENSE                          # MIT
├── README.md
└── plugins/
    └── exgen/
        ├── .claude-plugin/
        │   └── plugin.json              # plugin manifest (name, version, author)
        ├── skills/                      # -> /exgen:<skill-name>
        │   ├── structured-prompt/
        │   │   └── SKILL.md             # one feature -> one reviewable prompt
        │   └── user-story/
        │       └── SKILL.md             # full story workflow -> prompt per task
        ├── agents/                      # subagents Claude can delegate to
        │   ├── code-explorer.md
        │   └── code-architect.md
        └── README.md
```

`.claude-plugin/` only ever contains manifest files (`marketplace.json` at the
repo root, `plugin.json` inside the plugin). Every other directory (`skills/`,
`agents/`, `commands/`, `hooks/`, etc.) lives at the plugin root, one level up.

## Develop and test locally

Load the plugin directly from disk without installing it:

```bash
claude --plugin-dir ./plugins/exgen
```

Then try the workflow skill:

```
/exgen:user-story Add rate limiting to the password reset endpoint
```

or generate a single prompt directly:

```
/exgen:structured-prompt
```

After editing any file, run `/reload-plugins` inside the session to pick up
the change without restarting.

## Validate before sharing

```bash
claude plugin validate ./plugins/exgen   # checks the plugin manifest + components
claude plugin validate .                 # checks the marketplace.json too
```

## Install once it's on GitHub

Push this repository to GitHub, then anyone can register it as a marketplace
and install the plugin from it:

```
/plugin marketplace add trungleque/exgen-plugin
/plugin install exgen@exgen
```

Team members can also be auto-prompted to install it by adding to their
project's `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "exgen": {
      "source": { "source": "github", "repo": "trungleque/exgen-plugin" }
    }
  },
  "enabledPlugins": {
    "exgen@exgen": true
  }
}
```

## License

[MIT](LICENSE) — free for everyone to use, modify, and share.

Reference: [Create plugins](https://code.claude.com/docs/en/plugins) ·
[Plugins reference](https://code.claude.com/docs/en/plugins-reference) ·
[Create a marketplace](https://code.claude.com/docs/en/plugin-marketplaces)
