# exgen

A Claude Code plugin marketplace containing the `exgen` plugin, which bundles
skills, commands, and agents so they can be installed on any machine with one
command.

## Repository layout

```
.
├── .claude-plugin/
│   └── marketplace.json        # marketplace catalog — lists the exgen plugin
└── plugins/
    └── exgen/
        ├── .claude-plugin/
        │   └── plugin.json     # plugin manifest (name, version, author)
        ├── skills/              # model-invoked skills   -> /exgen:<skill-name>
        │   └── example-skill/
        │       └── SKILL.md
        ├── commands/            # slash commands          -> /exgen:<command-name>
        │   └── hello.md
        ├── agents/               # subagents Claude can delegate to
        │   └── example-agent.md
        └── README.md
```

`.claude-plugin/` only ever contains manifest files (`plugin.json`,
`marketplace.json`). Every other directory (`skills/`, `commands/`, `agents/`,
`hooks/`, etc.) lives at the plugin root, one level up.

## Develop and test locally

Load the plugin directly from disk without installing it:

```bash
claude --plugin-dir ./plugins/exgen
```

Then try the bundled command:

```
/exgen:hello Ada
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
/plugin marketplace add <your-github-username>/exgen
/plugin install exgen@exgen
```

(Replace `<your-github-username>/exgen` with wherever you push this repo.)

Team members can also be auto-prompted to install it by adding to their
project's `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "exgen": {
      "source": { "source": "github", "repo": "<your-github-username>/exgen" }
    }
  },
  "enabledPlugins": {
    "exgen@exgen": true
  }
}
```

## Next steps

- [ ] Replace the example skill, command, and agent in `plugins/exgen/` with
      real functionality (see the checklist inside each file)
- [ ] Set your name in `plugins/exgen/.claude-plugin/plugin.json` → `author`
- [ ] Bump `version` in both `plugin.json` and `marketplace.json` on releases
- [ ] Add a `LICENSE` file
- [ ] Push to GitHub and run `claude plugin validate .` in CI

Reference: [Create plugins](https://code.claude.com/docs/en/plugins) ·
[Plugins reference](https://code.claude.com/docs/en/plugins-reference) ·
[Create a marketplace](https://code.claude.com/docs/en/plugin-marketplaces)
