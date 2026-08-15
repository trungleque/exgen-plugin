package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

func newInstallCommand() *cobra.Command {
	var flags sourceFlags
	var dryRun bool

	c := &cobra.Command{
		Use:   "install <target>... ",
		Short: "Install a plugin's components into one or more coding tools",
		Long: strings.TrimSpace(`
Install a Claude Code plugin's skills, agents and commands into the given
targets. Pass "all" to install everywhere.

Each component is translated into the target's own conventions, and anything
that cannot cross over unchanged is reported on the line it affects.`),
		Example: strings.TrimSpace(`
  exgen install claude
  exgen install pi codex
  exgen install all --dry-run
  exgen install antigravity --project
  exgen install pi --from ../some-plugin --only skills`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targets, err := target.Resolve(args)
			if err != nil {
				return err
			}
			p, err := flags.load()
			if err != nil {
				return err
			}
			ctx, err := flags.context()
			if err != nil {
				return err
			}

			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s %s  (%s)\n", p.Name, p.Version, p.Source)
			fmt.Fprintf(w, "%d skills, %d agents, %d commands\n",
				len(p.Skills), len(p.Agents), len(p.Commands))

			manifest, err := install.LoadManifest(manifestPath(ctx))
			if err != nil {
				return fmt.Errorf("read install manifest: %w", err)
			}

			var counter tally
			for _, tg := range targets {
				pl, err := tg.Plan(p, ctx)
				if err != nil {
					return fmt.Errorf("%s: %w", tg.ID(), err)
				}
				results, err := install.Apply(pl, dryRun)
				if err != nil {
					return fmt.Errorf("%s: %w", tg.ID(), err)
				}
				// Recording what was written is what lets a later uninstall
				// tell a local edit apart from a version difference.
				manifest.Record(p.Name, tg.ID(), pl.Actions)
				reportPlan(w, pl, results, ctx.Home, &counter)
			}

			if !dryRun {
				if err := manifest.Save(); err != nil {
					return fmt.Errorf("write install manifest: %w", err)
				}
			}
			fmt.Fprintf(w, "\n%s\n", counter.summary(dryRun))
			return nil
		},
	}

	flags.register(c)
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be written without touching the disk")
	return c
}
