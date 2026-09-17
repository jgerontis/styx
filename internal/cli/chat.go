package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jgerontis/styx/internal/config"
	"github.com/jgerontis/styx/internal/message"
	"github.com/jgerontis/styx/internal/permission"
	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/runtime"
	"github.com/jgerontis/styx/internal/skill"
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
	skills, err := discoverSkillsForChat(workspace.Root)
	if err != nil {
		return err
	}
	loader := tool.NewLoadSkill(skills)
	if err := tools.Register(loader); err != nil {
		return err
	}

	fmt.Fprintf(output, "%s connected.\n", p.Name())
	fmt.Fprintf(output, "Styx chat using %s/%s. Type /reset or /exit.\n", providerName, model)
	history := []message.Message{*message.NewTextMessage(message.RoleSystem, systemPrompt(workspace, skills.All()))}
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
		if err := runTurnWithLimits(ctx, output, scanner, p, model, tools, loader, &history, maxToolCalls, maxSameToolCalls); err != nil {
			return err
		}
	}
}

func runTurn(ctx context.Context, output io.Writer, input *bufio.Scanner, p provider.Provider, model string, tools *tool.Registry, history *[]message.Message) error {
	return runTurnWithLimits(ctx, output, input, p, model, tools, nil, history, config.DefaultLoopMaxIterations, config.DefaultLoopMaxSameToolCalls)
}

func runTurnWithLimits(ctx context.Context, output io.Writer, input *bufio.Scanner, p provider.Provider, model string, tools *tool.Registry, loader *tool.LoadSkill, history *[]message.Message, maxToolCalls, maxSameToolCalls int) error {
	guard := newToolCallGuard(maxToolCalls, maxSameToolCalls)
	failures := make(map[string]int)
	policy := permission.Policy{}
	for {
		fmt.Fprint(output, "Assistant: ")
		stream, err := p.Chat(ctx, provider.ChatRequest{Model: model, Messages: *history, Tools: tools.Definitions(), MaxTokens: 1024, Think: false})
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
			result, err := executeToolCall(ctx, input, output, tools, policy, call)
			if err != nil {
				result = fmt.Sprintf("tool error: %v", err)
				fmt.Fprintf(output, "Tool failed: %s\n", result)
				failures[call.ToolName]++
				if isMutationTool(call.ToolName) && failures[call.ToolName] >= 2 {
					fmt.Fprintf(output, "Agent stopped this turn: %s failed twice; re-read the file and try again.\n", call.ToolName)
					return nil
				}
			} else {
				failures[call.ToolName] = 0
			}
			*history = append(*history, message.Message{
				Role:     message.RoleTool,
				ToolName: call.ToolName,
				Content:  []message.ContentPart{{Type: "text", Text: result}},
			})
			if err == nil && call.ToolName == "load_skill" && loader != nil {
				if name, ok := call.Args["name"].(string); ok {
					if selected, ok := loader.Skill(name); ok {
						policy.AllowedTools = append(policy.AllowedTools, selected.AllowedToolNames()...)
					}
				}
			}
		}
	}
}

func isMutationTool(toolName string) bool {
	return toolName == "edit_file" || toolName == "write_file" || toolName == "run_command"
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
	if call.ToolName != "edit_file" && call.ToolName != "write_file" {
		return tools.Execute(ctx, call.ToolName, call.Args)
	}
	return confirmMutation(ctx, input, output, tools, call)
}

func confirmMutation(ctx context.Context, input *bufio.Scanner, output io.Writer, tools *tool.Registry, call message.ToolCall) (string, error) {
	previewArgs := make(map[string]interface{}, len(call.Args))
	for key, value := range call.Args {
		previewArgs[key] = value
	}
	previewArgs["apply"] = false
	preview, err := tools.Execute(ctx, call.ToolName, previewArgs)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(output, "%s\nApply this change? [y/N] ", preview)
	if !input.Scan() {
		if err := input.Err(); err != nil {
			return "", err
		}
		return "Change was not approved.", nil
	}
	if strings.ToLower(strings.TrimSpace(input.Text())) != "y" {
		return "Change was not approved.", nil
	}
	applyArgs := make(map[string]interface{}, len(call.Args)+1)
	for key, value := range call.Args {
		applyArgs[key] = value
	}
	applyArgs["apply"] = true
	return tools.Execute(ctx, call.ToolName, applyArgs)
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

func systemPrompt(workspace tool.WorkspaceSummary, skills []skill.Skill) string {
	return fmt.Sprintf(`You are Styx, a precise terminal coding agent. Ground repository answers in tool results and distinguish facts from inferences. Preserve the user's requested document structure: when asked to add a paragraph, insert a standalone paragraph rather than modifying a nearby bullet or sentence unless explicitly asked to do so. Choose the narrowest tool that answers the question: map with list_files, discover literals with search_text, inspect syntax with search_structure, then read only relevant lines. Use one edit_file call per change with exact old_string copied from read_file (without its line prefix) and the desired new_string; include surrounding lines when the old text is not unique. For edit_file and write_file, propose the change only; Styx handles preview and approval. Use focused tests or builds to validate changes. Mutating tools and commands require user approval. Load a Skill only when its catalog description matches the task; do not assume its instructions before loading it. Keep answers concise unless the user asks for detail.

Workspace root: %s
Project: %s
Repository description: %s

Available Skills:
%s`, workspace.Root, workspace.ProjectName, workspace.Description, formatSkillCatalog(skills))
}

func discoverSkillsForChat(workspaceRoot string) (*skill.Registry, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	registry := skill.NewRegistryWithBuiltins(skill.Builtins(), filepath.Join(home, ".styx", "skills"), filepath.Join(workspaceRoot, ".styx", "skills"))
	if err := registry.Discover(); err != nil {
		return nil, err
	}
	return registry, nil
}

func formatSkillCatalog(skills []skill.Skill) string {
	if len(skills) == 0 {
		return "(none)"
	}
	lines := make([]string, 0, len(skills))
	for _, available := range skills {
		lines = append(lines, "- "+available.Name+": "+available.Description)
	}
	return strings.Join(lines, "\n")
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
