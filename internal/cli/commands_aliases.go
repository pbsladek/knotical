package cli

import (
	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newAliasesCommand() *cobra.Command {
	aliasStore := store.JSONMapStore{Path: config.AliasesFilePath()}
	cmd := &cobra.Command{Use: "aliases", Short: "Manage model aliases"}
	cmd.AddCommand(
		&cobra.Command{
			Use:  "set <alias> <model>",
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error {
				aliases, err := aliasStore.Load()
				if err != nil {
					return err
				}
				aliases[args[0]] = args[1]
				return aliasStore.Save(aliases)
			},
		},
		&cobra.Command{
			Use:  "remove <alias>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				aliases, err := aliasStore.Load()
				if err != nil {
					return err
				}
				delete(aliases, args[0])
				return aliasStore.Save(aliases)
			},
		},
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				aliases, err := aliasStore.Load()
				if err != nil {
					return err
				}
				for alias, target := range aliases {
					output.ListItem(alias, target)
				}
				return nil
			},
		},
	)
	return cmd
}
