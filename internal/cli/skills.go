package cli

import (
	"fmt"
	"path/filepath"

	"github.com/jgerontis/styx/internal/runtime"
	"github.com/jgerontis/styx/internal/skill"
	"github.com/jgerontis/styx/internal/tool"
	"github.com/spf13/cobra"
)

func newSkillsCommand(rt *runtime.Runtime) *cobra.Command {
	command := &cobra.Command{
		Use:   "skills",
		Short: "List and inspect available Agent Skills",
	}
	command.AddCommand(newSkillsListCommand(rt), newSkillsShowCommand(rt))
	return command
}

func newSkillsListCommand(rt *runtime.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List skill names and descriptions",
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := discoverSkills(rt)
			if err != nil {
				return err
			}
			for _, available := range registry.All() {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", available.Name, available.Description)
			}
			return nil
		},
	}
}

func newSkillsShowCommand(rt *runtime.Runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show instructions for one skill",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			registry, err := discoverSkills(rt)
			if err != nil {
				return err
			}
			available, ok := registry.Get(args[0])
			if !ok {
				return fmt.Errorf("skill %q not found", args[0])
			}
			instructions, err := available.Instructions()
			if err != nil {
				return err
			}
			fmt.Fprint(cmd.OutOrStdout(), instructions)
			return nil
		},
	}
}

func discoverSkills(rt *runtime.Runtime) (*skill.Registry, error) {
	workspace, err := tool.InspectWorkspace(".")
	if err != nil {
		return nil, err
	}
	registry := skill.NewRegistryWithBuiltins(skill.Builtins(), rt.Config.SkillsDir, filepath.Join(workspace.Root, ".styx", "skills"))
	if err := registry.Discover(); err != nil {
		return nil, err
	}
	return registry, nil
}
