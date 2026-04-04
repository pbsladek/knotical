package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newChatsCommand() *cobra.Command {
	chatStore := store.ChatStore{Dir: config.ChatCacheDir()}
	cmd := &cobra.Command{Use: "chats", Short: "Manage chat sessions"}
	cmd.AddCommand(
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				names, err := chatStore.List()
				if err != nil {
					return err
				}
				for _, name := range names {
					output.Println(name)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:  "show <session>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				session, err := chatStore.LoadOrCreate(args[0])
				if err != nil {
					return err
				}
				for _, msg := range session.Messages {
					output.Println(fmt.Sprintf("[%s] %s\n", msg.Role, msg.Content))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:  "delete <session>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				_, err := chatStore.Delete(args[0])
				return err
			},
		},
	)
	return cmd
}
