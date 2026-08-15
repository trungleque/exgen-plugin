package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trungleque/exgen-plugin/cli/internal/plugin"
	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

func newListCommand() *cobra.Command {
	var flags sourceFlags

	c := &cobra.Command{
		Use:   "list [target]...",
		Short: "Show a plugin's components and how each target handles them",
		Long: strings.TrimSpace(`
List the components of a plugin. With one or more targets, also show where each
component kind lands in that tool and which ones need translating to get there.`),
		Example: strings.TrimSpace(`
  exgen list
  exgen list pi
  exgen list all --project`),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := flags.load()
			if err != nil {
				return err
			}
			ctx, err := flags.context()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s %s\n%s\n", p.Name, p.Version, p.Source)
			if p.Description != "" {
				fmt.Fprintf(w, "\n%s\n", p.Description)
			}

			counts := map[plugin.Kind]int{}
			for _, k := range plugin.AllKinds {
				counts[k] = p.Count(k)
			}

			fmt.Fprintf(w, "\ncomponents\n")
			for _, s := range p.Skills {
				extra := ""
				if len(s.Files) > 0 {
					extra = fmt.Sprintf("  +%d bundled file(s)", len(s.Files))
				}
				if s.Frontmatter.IsTrue("disable-model-invocation") {
					extra += "  [explicit invocation only]"
				}
				fmt.Fprintf(w, "  skill    %-22s %s%s\n", s.Name, truncate(s.Description, 60), extra)
			}
			for _, a := range p.Agents {
				fmt.Fprintf(w, "  agent    %-22s %s\n", a.Name, truncate(a.Description, 60))
			}
			for _, c := range p.Commands {
				fmt.Fprintf(w, "  command  %-22s %s\n", c.Name, truncate(c.Description, 60))
			}

			if len(args) == 0 {
				fmt.Fprintf(w, "\ntargets: %s\n", strings.Join(target.IDs(), ", "))
				fmt.Fprintf(w, "run `exgen list <target>` to see how these map\n")
				return nil
			}

			targets, err := target.Resolve(args)
			if err != nil {
				return err
			}
			for _, tg := range targets {
				describeSupport(w, tg, ctx, counts)
			}
			return nil
		},
	}

	flags.register(c)
	return c
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
