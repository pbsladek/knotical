package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newKeysCommand() *cobra.Command {
	manager := store.NewKeyManager(config.KeysFilePath())
	cmd := &cobra.Command{Use: "keys", Short: "Manage API keys"}
	getCmd := &cobra.Command{
		Use:  "get <provider>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, ok, err := manager.Get(args[0])
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("no key found for provider %q", args[0])
			}
			reveal, _ := cmd.Flags().GetBool("reveal")
			if reveal {
				output.Println(key)
			} else {
				output.Println(store.MaskKey(key))
			}
			return nil
		},
	}
	getCmd.Flags().Bool("reveal", false, "Print the full key")
	cmd.AddCommand(
		&cobra.Command{
			Use:  "set <provider>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				value, err := store.PromptHidden(fmt.Sprintf("Enter API key for %s: ", args[0]))
				if err != nil {
					return err
				}
				return manager.Set(args[0], strings.TrimSpace(value))
			},
		},
		getCmd,
		&cobra.Command{
			Use:  "remove <provider>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				removed, err := manager.Remove(args[0])
				if err != nil {
					return err
				}
				if !removed {
					return fmt.Errorf("no stored key found for %q", args[0])
				}
				return nil
			},
		},
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				providers, err := manager.ListStored()
				if err != nil {
					return err
				}
				for _, provider := range providers {
					key, _, _ := manager.Get(provider)
					output.ListItem(provider, store.MaskKey(key))
				}
				return nil
			},
		},
		&cobra.Command{
			Use: "path",
			Run: func(cmd *cobra.Command, args []string) { output.Println(config.KeysFilePath()) },
		},
	)
	return cmd
}
