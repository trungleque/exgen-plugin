# exgen

A Claude Code plugin bundling skills, commands, and agents.

## Components

| Type | Location | Invocation |
| :--- | :--- | :--- |
| Skill | `skills/example-skill/SKILL.md` | Auto-invoked by Claude, or `/exgen:example-skill` |
| Command | `commands/hello.md` | `/exgen:hello [name]` |
| Agent | `agents/example-agent.md` | Auto-delegated by Claude, or `@exgen-example-agent` |

All three are currently placeholders — see the checklist in each file for what to fill in.

## Add more components

- **More skills**: add a new folder under `skills/`, each with its own `SKILL.md`
- **More commands**: add a new `.md` file under `commands/`
- **More agents**: add a new `.md` file under `agents/`
- **Hooks**: add `hooks/hooks.json` at the plugin root
- **MCP servers**: add `.mcp.json` at the plugin root
- **LSP servers**: add `.lsp.json` at the plugin root

See the [Plugins reference](https://code.claude.com/docs/en/plugins-reference) for full schemas.
