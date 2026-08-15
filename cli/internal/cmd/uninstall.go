package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/trungleque/exgen-plugin/cli/internal/install"
	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

func newUninstallCommand() *cobra.Command {
	var flags sourceFlags
	var dryRun, force bool

	c := &cobra.Command{
		Use:   "uninstall <target>...",
		Short: "Remove previously installed components from one or more coding tools",
		Long: strings.TrimSpace(`
Remove the files an install of this plugin would have written.

Removal is content-checked rather than tracked in a manifest: a file whose
contents no longer match what this plugin would install is assumed to be
hand-edited and is left in place. Pass --force to delete it anyway.`),
		Example: strings.TrimSpace(`
  exgen uninstall pi
  exgen uninstall all --dry-run`),
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
			fmt.Fprintf(w, "removing %s %s\n", p.Name, p.Version)

			manifest, err := install.LoadManifest(manifestPath(ctx))
			if err != nil {
				return fmt.Errorf("read install manifest: %w", err)
			}

			var counter tally
			var unrecorded []string
			for _, tg := range targets {
				pl, err := tg.Plan(p, ctx)
				if err != nil {
					return fmt.Errorf("%s: %w", tg.ID(), err)
				}

				// Prefer what the install actually recorded. Falling back to
				// the plan means comparing against what *this* build would
				// write, which reads a version difference as a local edit.
				entries := manifest.Entries(tg.ID())
				if len(entries) == 0 {
					entries = pl.Removals()
					unrecorded = append(unrecorded, tg.ID())
				}

				results, err := install.Remove(entries, dryRun, force)
				if err != nil {
					return fmt.Errorf("%s: %w", tg.ID(), err)
				}
				if !dryRun {
					manifest.Forget(tg.ID())
				}

				// Translation notes describe an install, not a removal.
				pl.Notes, pl.Actions = nil, nil
				reportPlan(w, pl, results, ctx.Home, &counter)
			}

			if !dryRun {
				if err := manifest.Save(); err != nil {
					return fmt.Errorf("write install manifest: %w", err)
				}
			}

			fmt.Fprintf(w, "\n%s\n", counter.summary(dryRun))
			if len(unrecorded) > 0 {
				fmt.Fprintf(w, "%s no install record for %s — fell back to the paths this version would write\n",
					markWarn, strings.Join(unrecorded, ", "))
			}
			if counter.modified > 0 && !force {
				fmt.Fprintf(w, "%s %d file(s) changed since they were installed and were kept; use --force to remove them\n",
					markWarn, counter.modified)
			}
			return nil
		},
	}

	flags.register(c)
	c.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without touching the disk")
	c.Flags().BoolVar(&force, "force", false, "remove files even when they have local edits")
	return c
}
