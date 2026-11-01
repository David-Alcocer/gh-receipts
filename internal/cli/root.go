// Package cli defines the gh-receipts command tree.
package cli

import (
	"runtime/debug"

	"github.com/spf13/cobra"
)

// Execute runs the root command against the process arguments.
func Execute() error {
	return newRootCmd(versionFromBuildInfo(debug.ReadBuildInfo())).Execute()
}

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "gh-receipts",
		Short:         "Know what your team actually shipped",
		Version:       version,
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		Annotations:   map[string]string{cobra.CommandDisplayNameAnnotation: "gh receipts"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newTodayCmd(defaultTodayDeps()))
	return cmd
}
