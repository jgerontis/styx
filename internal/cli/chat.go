package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jgerontis/styx/internal/config"
	"github.com/jgerontis/styx/internal/message"
	"github.com/jgerontis/styx/internal/permission"
	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/runtime"
	"github.com/jgerontis/styx/internal/tool"
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
			return runChatWithLimits(cmd.Context(), os.Stdin, os.Stdout, rt.Providers, rt.Config.Provider, model, rt.Config.LoopMaxIterations, rt.Config.LoopMaxSameToolCalls)
		},
	}

	cmd.Flags().StringVarP(&model, "model", "m", "", "model to use")
	return cmd
}

func runChat(ctx context.Context, input io.Reader, output io.Writer, providers *provider.Registry, providerName, model string) error {
	return runChatWithLimits(ctx, input, output, providers, providerName, model, config.DefaultLoopMaxIterations, config.DefaultLoopMaxSameToolCalls)
}

func runChatWithLimits(ctx context.Context, input io.Reader, output io.Writer, providers *provider.Registry, providerName, model string, maxToolCalls, maxSameToolCalls int) error {
	p, err := providers.Get(providerName)
	if err != nil {
		return err
	}
	if err := p.Connect(ctx); err != nil {
		return fmt.Errorf("connect to %s: %w", providerName, err)
	}
	defer p.Disconnect(context.Background())
	if err := confirmModel(ctx, p, model); err != nil {
		return err
	}
	workspace, err := tool.InspectWorkspace(".")
	if err != nil {
		return err
	}
	tools := tool.NewRegistry()
	if err := tools.Register(tool.NewReadFile(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewEditFile(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewWriteFile(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewRunCommand(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewListFiles(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewSearchText(workspace.Root)); err != nil {
		return err
	}
	if err := tools.Register(tool.NewSearchStructure(workspace.Root)); err != nil {
		return err
	}

	fmt.Fprintf(output, "%s connected.\n", p.Name())
	fmt.Fprintf(output, "Styx chat using %s/%s. Type /reset or /exit.\n", providerName, model)
	history := []message.Message{*message.NewTextMessage(message.RoleSystem, systemPrompt(workspace))}
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
			history = history[:1]
			fmt.Fprintln(output, "Conversation reset.")
			continue
		}

		history = append(history, *message.NewTextMessage(message.RoleUser, prompt))
		if err := runTurnWithLimits(ctx, output, scanner, p, model, tools, &history, maxToolCalls, maxSameToolCalls); err != nil {
			return err
		}
	}
}

func runTurn(ctx context.Context, output io.Writer, input *bufio.Scanner, p provider.Provider, model string, tools *tool.Registry, history *[]message.Message) error {
	return runTurnWithLimits(ctx, output, input, p, model, tools, history, config.DefaultLoopMaxIterations, config.DefaultLoopMaxSameToolCalls)
}

func runTurnWithLimits(ctx context.Context, output io.Writer, input *bufio.Scanner, p provider.Provider, model string, tools *tool.Registry, history *[]message.Message, maxToolCalls, maxSameToolCalls int) error {
	guard := newToolCallGuard(maxToolCalls, maxSameToolCalls)
	for {
		fmt.Fprint(output, "Assistant: ")
		stream, err := p.Chat(ctx, provider.ChatRequest{Model: model, Messages: *history, Tools: tools.Definitions()})
		if err != nil {
			return fmt.Errorf("chat with %s: %w", p.Name(), err)
		}

		response, err := streamToMessage(stream, output)
		if err != nil {
			return err
		}
		*history = append(*history, *response)
		if len(response.ToolCalls) == 0 {
			return nil
		}

		for _, call := range response.ToolCalls {
			if err := guard.Observe(call.ToolName); err != nil {
				fmt.Fprintf(output, "Agent stopped this turn: %v\n", err)
				return nil
			}
			fmt.Fprintf(output, "Using %s...\n", call.ToolName)
			result, err := executeToolCall(ctx, input, output, tools, permission.Policy{}, call)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
			}
			*history = append(*history, message.Message{
				Role:     message.RoleTool,
				ToolName: call.ToolName,
				Content:  []message.ContentPart{{Type: "text", Text: result}},
			})
		}
	}
}

type toolCallGuard struct {
	maxCalls        int
	maxSameTool     int
	calls           int
	previousTool    string
	consecutiveSame int
}

func newToolCallGuard(maxCalls, maxSameTool int) *toolCallGuard {
	return &toolCallGuard{maxCalls: maxCalls, maxSameTool: maxSameTool}
}

func (g *toolCallGuard) Observe(toolName string) error {
	g.calls++
	if g.calls > g.maxCalls {
		return fmt.Errorf("tool-call limit of %d reached", g.maxCalls)
	}
	if toolName == g.previousTool {
		g.consecutiveSame++
	} else {
		g.previousTool = toolName
		g.consecutiveSame = 1
	}
	if g.consecutiveSame > g.maxSameTool {
		return fmt.Errorf("%s called %d times in a row", toolName, g.consecutiveSame)
	}
	return nil
}

func executeToolCall(ctx context.Context, input *bufio.Scanner, output io.Writer, tools *tool.Registry, policy permission.Policy, call message.ToolCall) (string, error) {
	if policy.Check(call.ToolName) == permission.Allow {
		return tools.Execute(ctx, call.ToolName, call.Args)
	}
	if call.ToolName == "run_command" {
		return confirmCommand(ctx, input, output, tools, call)
	}
	if call.Args["apply"] != true {
		return tools.Execute(ctx, call.ToolName, call.Args)
	}

	previewArgs := make(map[string]interface{}, len(call.Args))
	for key, value := range call.Args {
		previewArgs[key] = value
	}
	previewArgs["apply"] = false
	preview, err := tools.Execute(ctx, call.ToolName, previewArgs)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(output, "%s\nApply this edit? [y/N] ", preview)
	if !input.Scan() {
		if err := input.Err(); err != nil {
			return "", err
		}
		return "Edit was not approved.", nil
	}
	if strings.ToLower(strings.TrimSpace(input.Text())) != "y" {
		return "Edit was not approved.", nil
	}
	return tools.Execute(ctx, call.ToolName, call.Args)
}

func confirmCommand(ctx context.Context, input *bufio.Scanner, output io.Writer, tools *tool.Registry, call message.ToolCall) (string, error) {
	program, _ := call.Args["program"].(string)
	fmt.Fprintf(output, "Run command %q? [y/N] ", program)
	if !input.Scan() {
		if err := input.Err(); err != nil {
			return "", err
		}
		return "Command was not approved.", nil
	}
	if strings.ToLower(strings.TrimSpace(input.Text())) != "y" {
		return "Command was not approved.", nil
	}
	return tools.Execute(ctx, call.ToolName, call.Args)
}

func systemPrompt(workspace tool.WorkspaceSummary) string {
	return fmt.Sprintf(`You are Styx, a terminal coding agent. Be precise and grounded in the workspace facts supplied below. For repository questions, state what is known and distinguish it from inference. Use list_files to map an unfamiliar layout without reading content. Use search_text first to find names, strings, configuration, documentation, or relevant files and infer the language from file extensions. Use search_structure only for syntax-shaped questions such as declarations, imports, calls, or components. Its pattern must be valid code in the required language; use $NAME for one node and $$$NODES for zero or more nodes. Start with the smallest pattern that answers the question. If it has no matches, remove one constraint and retry once, then use search_text. Use read_file for only the necessary line range. Each returned line has a line:hash anchor; retain fresh anchors for future edits and re-read after any stale-anchor error. To edit one file, use one edit_file call containing every change in its operations array, with fresh anchors. Styx previews each patch and asks once before writing. Use write_file only for a new file; it refuses to overwrite. Use run_command for focused test or build commands; it always asks for approval. Keep answers concise unless the user asks for detail.

Workspace root: %s
Project: %s
Repository description: %s`, workspace.Root, workspace.ProjectName, workspace.Description)
}

func confirmModel(ctx context.Context, p provider.Provider, model string) error {
	models, err := p.Models(ctx)
	if err != nil {
		return fmt.Errorf("list models from %s: %w", p.Name(), err)
	}

	for _, available := range models {
		if available.ID == model {
			return nil
		}
	}

	return fmt.Errorf("model %q is not available from %s; install it with `ollama pull %s`", model, p.Name(), model)
}

func streamToMessage(stream provider.StreamReader, output io.Writer) (*message.Message, error) {
	defer stream.Close()

	var content strings.Builder
	var toolCalls []message.ToolCall
	flusher, canFlush := output.(interface{ Flush() error })
	for {
		delta, err := stream.Recv()
		if err == io.EOF {
			fmt.Fprintln(output)
			response := message.NewTextMessage(message.RoleAssistant, content.String())
			response.ToolCalls = toolCalls
			return response, nil
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
		if delta.Type == "tool_call_end" && delta.ToolCall != nil {
			var args map[string]interface{}
			if err := json.Unmarshal(delta.ToolCall.Args, &args); err != nil {
				return nil, fmt.Errorf("parse %s arguments: %w", delta.ToolCall.ToolName, err)
			}
			toolCalls = append(toolCalls, message.ToolCall{ID: delta.ToolCall.ID, ToolName: delta.ToolCall.ToolName, Args: args})
		}
	}
}
