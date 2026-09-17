package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jgerontis/styx/internal/runtime"
)

func main() {
	ctx := context.Background()

	rt, err := runtime.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize runtime: %v\n", err)
		os.Exit(1)
	}

	rt.Logger.InfoContext(ctx, "styx started")

	// Placeholder: Cobra root command will be wired in Phase 6
	fmt.Println("Styx v1 - Terminal AI Agent Harness")
	fmt.Println("Provider:", rt.Config.Provider)
	fmt.Println("Model:", rt.Config.Model)
}
