package cli

import (
	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newFragmentsCommand() *cobra.Command {
	fragmentStore := store.FragmentStore{Dir: config.FragmentsDir()}
	cmd := &cobra.Command{Use: "fragments", Short: "Manage reusable fragments"}
	cmd.AddCommand(
		&cobra.Command{
			Use:  "set <name> <content>",
			Args: cobra.ExactArgs(2),
			RunE: func(cmd *cobra.Command, args []string) error { return fragmentStore.Save(args[0], args[1]) },
		},
		&cobra.Command{
			Use:  "get <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				fragment, err := fragmentStore.Load(args[0])
				if err != nil {
					return err
				}
				output.Println(fragment.Content)
				return nil
			},
		},
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				fragments, err := fragmentStore.List()
				if err != nil {
					return err
				}
				for _, fragment := range fragments {
					output.ListItem(fragment.Name, fragment.Description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:  "delete <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error { return fragmentStore.Delete(args[0]) },
		},
	)
	return cmd
}
