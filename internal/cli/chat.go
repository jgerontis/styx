package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jgerontis/styx/internal/message"
	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/runtime"
	"github.com/spf13/cobra"
)

func newChatCommand(rt *runtime.Runtime) *cobra.Command {
	var model string

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Start an interactive chat session",
		RunE: func(cmd *cobra.Command, args []string) error {
			if model == "" {
				model = rt.Config.Model
			}
			return runChat(cmd.Context(), os.Stdin, os.Stdout, rt.Providers, rt.Config.Provider, model)
		},
	}

	cmd.Flags().StringVarP(&model, "model", "m", "", "model to use")
	return cmd
}

func runChat(ctx context.Context, input io.Reader, output io.Writer, providers *provider.Registry, providerName, model string) error {
	p, err := providers.Get(providerName)
	if err != nil {
		return err
	}
	if err := p.Connect(ctx); err != nil {
		return fmt.Errorf("connect to %s: %w", providerName, err)
	}
	defer p.Disconnect(context.Background())

	fmt.Fprintf(output, "Styx chat using %s/%s. Type /reset or /exit.\n", providerName, model)
	history := make([]message.Message, 0)
	scanner := bufio.NewScanner(input)

	for {
		fmt.Fprint(output, "> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return err
			}
			return nil
		}

		prompt := strings.TrimSpace(scanner.Text())
		switch prompt {
		case "":
			continue
		case "/exit", "/quit":
			return nil
		case "/reset":
			history = history[:0]
			fmt.Fprintln(output, "Conversation reset.")
			continue
		}

		history = append(history, *message.NewTextMessage(message.RoleUser, prompt))
		fmt.Fprint(output, "Assistant: ")
		stream, err := p.Chat(ctx, provider.ChatRequest{Model: model, Messages: history})
		if err != nil {
			return fmt.Errorf("chat with %s: %w", providerName, err)
		}

		response, err := streamToMessage(stream, output)
		if err != nil {
			return err
		}
		history = append(history, *response)
	}
}

func streamToMessage(stream provider.StreamReader, output io.Writer) (*message.Message, error) {
	defer stream.Close()

	var content strings.Builder
	flusher, canFlush := output.(interface{ Flush() error })
	for {
		delta, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintln(output)
			return message.NewTextMessage(message.RoleAssistant, content.String()), nil
		}
		if err != nil {
			return nil, err
		}
		if delta.Type == "text" {
			fmt.Fprint(output, delta.Text)
			if canFlush {
				if err := flusher.Flush(); err != nil {
					return nil, err
				}
			}
			content.WriteString(delta.Text)
		}
		if delta.Type == "error" {
			return nil, delta.Error
		}
	}
}
