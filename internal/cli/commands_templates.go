package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
	"github.com/pbsladek/knotical/internal/store"
)

func newTemplatesCommand() *cobra.Command {
	templateStore := store.TemplateStore{Dir: config.TemplatesDir()}
	cmd := &cobra.Command{Use: "templates", Short: "Manage templates"}
	createCmd := &cobra.Command{
		Use:  "create <name>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			systemPrompt, _ := cmd.Flags().GetString("system")
			modelID, _ := cmd.Flags().GetString("model")
			description, _ := cmd.Flags().GetString("description")
			return templateStore.Save(store.Template{Name: args[0], SystemPrompt: systemPrompt, Model: modelID, Description: description})
		},
	}
	createCmd.Flags().String("system", "", "System prompt")
	createCmd.Flags().String("model", "", "Model")
	createCmd.Flags().String("description", "", "Description")
	cmd.AddCommand(
		&cobra.Command{
			Use: "list",
			RunE: func(cmd *cobra.Command, args []string) error {
				templates, err := templateStore.List()
				if err != nil {
					return err
				}
				for _, template := range templates {
					output.ListItem(template.Name, template.Description)
				}
				return nil
			},
		},
		&cobra.Command{
			Use:  "show <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				template, err := templateStore.Load(args[0])
				if err != nil {
					return err
				}
				output.Println(fmt.Sprintf("Name: %s\nModel: %s\nSystem:\n%s", template.Name, template.Model, template.SystemPrompt))
				return nil
			},
		},
		createCmd,
		&cobra.Command{
			Use:  "edit <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error {
				if !templateStore.Exists(args[0]) {
					if err := templateStore.Save(store.Template{Name: args[0]}); err != nil {
						return err
					}
				}
				editor := os.Getenv("EDITOR")
				if editor == "" {
					editor = "vi"
				}
				templatePath, err := templateStore.Path(args[0])
				if err != nil {
					return err
				}
				name, argv, err := editorCommandArgs(editor, templatePath)
				if err != nil {
					return err
				}
				command := exec.Command(name, argv...)
				command.Stdin = os.Stdin
				command.Stdout = os.Stdout
				command.Stderr = os.Stderr
				return command.Run()
			},
		},
		&cobra.Command{
			Use:  "delete <name>",
			Args: cobra.ExactArgs(1),
			RunE: func(cmd *cobra.Command, args []string) error { return templateStore.Delete(args[0]) },
		},
	)
	return cmd
}
