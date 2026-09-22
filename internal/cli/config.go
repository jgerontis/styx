package cli

import (
	"fmt"

	"github.com/jgerontis/styx/internal/runtime"
	"github.com/spf13/cobra"
)

func newConfigCommand(rt *runtime.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "View and persist Styx configuration",
	}
	command.AddCommand(newConfigShowCommand(rt), newConfigSetCommand(rt))
	return command
}

func newConfigShowCommand(rt *runtime.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the active configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "provider: %s\nmodel: %s\nbase-url: %s\nlog-level: %s\n", rt.Config.Provider, rt.Config.Model, rt.Config.BaseURL, rt.Config.LogLevel)
			return nil
		},
	}
}

func newConfigSetCommand(rt *runtime.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Persist a configuration value to ~/.styx/config.yaml so it applies without flags",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, value := args[0], args[1]
			switch key {
			case "provider":
				rt.Config.Provider = value
			case "model":
				rt.Config.Model = value
			case "base-url":
				rt.Config.BaseURL = value
			case "log-level":
				rt.Config.LogLevel = value
			default:
				return fmt.Errorf("unknown config key %q (want one of: provider, model, base-url, log-level)", key)
			}
			if err := rt.Config.SaveToFile(); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s = %s\n", key, value)
			return nil
		},
	}
}
