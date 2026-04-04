package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newRolesCommand() *cobra.Command {
	roleStore := store.RoleStore{Dir: config.RolesDir()}
	cmd := &cobra.Command{Use: "roles", Short: "Manage roles"}
	createCmd := &cobra.Command{
		Use:  "create <name>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			systemPrompt := ""
			if value, _ := cmd.Flags().GetString("system"); value != "" {
				systemPrompt = value
			} else {
				var err error
				systemPrompt, err = openEditorForPrompt()
				if err != nil {
					return err
				}
			}
			return roleStore.Save(store.Role{Name: args[0], SystemPrompt: systemPrompt, PrettifyMarkdown: true})
		},
	}
	createCmd.Flags().String("system", "", "System prompt")
	cmd.AddCommand(
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				roles, err := roleStore.List()
				if err != nil {
					return err
				}
				for _, role := range roles {
					output.ListItem(role.Name, role.Description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:  "show <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				role, err := roleStore.Load(args[0])
				if err != nil {
					return err
				}
				output.Println(fmt.Sprintf("Name: %s\nPrompt:\n%s", role.Name, role.SystemPrompt))
				return nil
			},
		},
		createCmd,
		&cobra.Command{
			Use:  "delete <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				return roleStore.Delete(args[0])
			},
		},
	)
	return cmd
}
