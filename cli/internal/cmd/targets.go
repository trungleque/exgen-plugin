package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/trungleque/exgen-plugin/cli/internal/target"
)

func newTargetsCommand() *cobra.Command {
	var flags sourceFlags

	c := &cobra.Command{
		Use:   "targets",
		Short: "List supported coding tools and where each writes its components",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, err := flags.context()
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "install scope: %s\n", ctx.Scope)
			for _, tg := range target.All() {
				describeSupport(w, tg, ctx, nil)
			}
			return nil
		},
	}

	c.Flags().BoolVar(&flags.global, "global", false, "show user-level paths (default)")
	c.Flags().BoolVar(&flags.project, "project", false, "show repository-level paths")
	return c
}
