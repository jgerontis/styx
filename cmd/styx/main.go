package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jgerontis/styx/internal/cli"
	"github.com/jgerontis/styx/internal/runtime"
)

func main() {
	ctx := context.Background()

	rt, err := runtime.New(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize runtime: %v\n", err)
		os.Exit(1)
	}

	if err := cli.NewRootCommand(rt).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
