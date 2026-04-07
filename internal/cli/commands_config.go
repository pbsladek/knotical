package cli

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/pbsladek/knotical/internal/config"
	"github.com/pbsladek/knotical/internal/output"
)

func newConfigCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "config", Short: "Inspect and edit configuration"}
	var force bool
	cmd.AddCommand(
		&cobra.Command{
			Use: "show",
			RunE: func(cmd *cobra.Command, args []string) error {
				cfg, err := config.Load()
				if err != nil {
					return err
				}
				var buf bytes.Buffer
				if err := toml.NewEncoder(&buf).Encode(cfg); err != nil {
					return err
				}
				output.Print(buf.String())
				return nil
			},
		},
		&cobra.Command{
			Use: "path",
			Run: func(cmd *cobra.Command, args []string) {
				output.Println(config.ConfigFilePath())
			},
		},
		&cobra.Command{
			Use: "edit",
			RunE: func(cmd *cobra.Command, args []string) error {
				if _, err := os.Stat(config.ConfigFilePath()); os.IsNotExist(err) {
					if err := config.Save(config.Default()); err != nil {
						return err
					}
				} else if err != nil {
					return err
				}
				return openEditorAtPath(config.ConfigFilePath())
			},
		},
		func() *cobra.Command {
			generateCmd := &cobra.Command{
				Use:  "generate [path]",
				Args: cobra.MaximumNArgs(1),
				RunE: func(cmd *cobra.Command, args []string) error {
					path := config.ConfigFilePath()
					if len(args) > 0 {
						path = args[0]
					}
					if err := writeConfigFile(path, config.Default(), force); err != nil {
						return err
					}
					output.Println(path)
					return nil
				},
			}
			generateCmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing config file")
			return generateCmd
		}(),
	)
	return cmd
}

func writeConfigFile(path string, cfg config.Config, force bool) error {
	if _, err := os.Stat(path); err == nil && !force {
		return os.ErrExist
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return toml.NewEncoder(file).Encode(cfg)
}
