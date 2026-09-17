package cli

import (
	"github.com/jgerontis/styx/internal/runtime"
	"github.com/spf13/cobra"
)

// NewRootCommand constructs the Styx command line interface.
func NewRootCommand(rt *runtime.Runtime) *cobra.Command {
	root := &cobra.Command{
		Use:   "styx",
		Short: "A terminal-based AI agent harness",
	}

	root.AddCommand(newChatCommand(rt))
	return root
}
