package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/jgerontis/styx/internal/message"
	"github.com/jgerontis/styx/internal/permission"
	"github.com/jgerontis/styx/internal/provider"
	"github.com/jgerontis/styx/internal/tool"
)

func TestRunChatStreamsAndResetsHistory(t *testing.T) {
	var chats []struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"models":[{"name":"test-model"}]}`))
		case "/api/chat":
			var request struct {
				Messages []struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode chat request: %v", err)
				return
			}
			chats = append(chats, request)
			w.Header().Set("Content-Type", "application/x-ndjson")
			_, _ = w.Write([]byte("{\"message\":{\"content\":\"hello \"}}\n"))
			_, _ = w.Write([]byte("{\"message\":{\"content\":\"from Styx\"}}\n"))
			_, _ = w.Write([]byte("{\"done\":true}\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	registry := provider.NewRegistry()
	if err := registry.Register("ollama", provider.NewOllamaProvider(server.URL)); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	input := bytes.NewBufferString("first prompt\nsecond prompt\n/reset\nthird prompt\n/exit\n")
	var output bytes.Buffer
	if err := runChat(context.Background(), input, &output, registry, "ollama", "test-model"); err != nil {
		t.Fatalf("run chat: %v", err)
	}

	if len(chats) != 3 {
		t.Fatalf("expected 3 chat requests, got %d", len(chats))
	}
	if len(chats[0].Messages) < 2 || chats[0].Messages[0].Role != "system" || !strings.Contains(chats[0].Messages[0].Content, "Project: styx") {
		t.Errorf("expected first request to include grounded workspace system prompt, got %#v", chats[0].Messages)
	}
	if len(chats[1].Messages) != 4 {
		t.Errorf("expected second request to include system prompt and prior turn, got %d messages", len(chats[1].Messages))
	}
	if len(chats[2].Messages) != 2 || chats[2].Messages[1].Content != "third prompt" {
		t.Errorf("expected reset request to retain system prompt and contain third prompt, got %#v", chats[2].Messages)
	}
	if !bytes.Contains(output.Bytes(), []byte("hello from Styx")) {
		t.Errorf("expected streamed response in output, got %q", output.String())
	}
	if !bytes.Contains(output.Bytes(), []byte("ollama connected.")) {
		t.Errorf("expected connection status in output, got %q", output.String())
	}
}

func TestRunChatRejectsUnavailableModel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"installed-model"}]}`))
	}))
	defer server.Close()

	registry := provider.NewRegistry()
	if err := registry.Register("ollama", provider.NewOllamaProvider(server.URL)); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	err := runChat(context.Background(), bytes.NewBufferString("/exit\n"), io.Discard, registry, "ollama", "missing-model")
	if err == nil || !strings.Contains(err.Error(), "ollama pull missing-model") {
		t.Fatalf("expected pull guidance for missing model, got %v", err)
	}
}

func TestSystemPromptPreservesRequestedDocumentStructure(t *testing.T) {
	prompt := systemPrompt(tool.WorkspaceSummary{}, nil)
	if !strings.Contains(prompt, "add a paragraph, insert a standalone paragraph") {
		t.Errorf("system prompt does not preserve paragraph requests: %q", prompt)
	}
}

func TestRunChatExecutesReadFileToolCall(t *testing.T) {
	var chatRequests []struct {
		Messages []struct {
			Role     string `json:"role"`
			Content  string `json:"content"`
			ToolName string `json:"tool_name"`
		} `json:"messages"`
		Tools []json.RawMessage `json:"tools"`
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"test-model"}]}`))
		case "/api/chat":
			var request struct {
				Messages []struct {
					Role     string `json:"role"`
					Content  string `json:"content"`
					ToolName string `json:"tool_name"`
				} `json:"messages"`
				Tools []json.RawMessage `json:"tools"`
			}
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			chatRequests = append(chatRequests, request)
			if len(chatRequests) == 1 {
				_, _ = w.Write([]byte(`{"message":{"tool_calls":[{"function":{"index":0,"name":"read_file","arguments":{"path":"README.md","start_line":1,"end_line":1}}}]}}` + "\n"))
				_, _ = w.Write([]byte(`{"done":true}` + "\n"))
				return
			}
			_, _ = w.Write([]byte(`{"message":{"content":"Styx is a terminal agent harness."}}` + "\n"))
			_, _ = w.Write([]byte(`{"done":true}` + "\n"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	registry := provider.NewRegistry()
	if err := registry.Register("ollama", provider.NewOllamaProvider(server.URL)); err != nil {
		t.Fatalf("register provider: %v", err)
	}

	var output bytes.Buffer
	if err := runChat(context.Background(), bytes.NewBufferString("describe the repository\n/exit\n"), &output, registry, "ollama", "test-model"); err != nil {
		t.Fatalf("run chat: %v", err)
	}

	if len(chatRequests) != 2 {
		t.Fatalf("expected tool follow-up request, got %d chat requests", len(chatRequests))
	}
	foundReadFile := false
	for _, definition := range chatRequests[0].Tools {
		if strings.Contains(string(definition), `"name":"read_file"`) {
			foundReadFile = true
			break
		}
	}
	if !foundReadFile {
		t.Errorf("expected read_file schema in first request, got %s", chatRequests[0].Tools)
	}
	last := chatRequests[1].Messages[len(chatRequests[1].Messages)-1]
	if last.Role != "tool" || last.ToolName != "read_file" || !strings.Contains(last.Content, "# Styx") {
		t.Errorf("expected README result in follow-up, got %#v", last)
	}
	if !strings.Contains(output.String(), "Styx is a terminal agent harness.") {
		t.Errorf("expected final answer, got %q", output.String())
	}
}

func TestExecuteToolCallRequiresApprovalForEdits(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tools := tool.NewRegistry()
	if err := tools.Register(tool.NewEditFile(root)); err != nil {
		t.Fatalf("register edit tool: %v", err)
	}
	call := message.ToolCall{ToolName: "edit_file", Args: map[string]interface{}{"path": "example.txt", "old_string": "before", "new_string": "after"}}
	var output bytes.Buffer

	result, err := executeToolCall(context.Background(), bufio.NewScanner(strings.NewReader("n\n")), &output, tools, permission.Policy{}, call)
	if err != nil || result != "Change was not approved." {
		t.Fatalf("result = %q, error = %v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "before\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
	if !strings.Contains(output.String(), "Preview:") {
		t.Errorf("expected preview, got %q", output.String())
	}
}

func TestExecuteToolCallAppliesApprovedEdit(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "example.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	tools := tool.NewRegistry()
	if err := tools.Register(tool.NewEditFile(root)); err != nil {
		t.Fatalf("register edit tool: %v", err)
	}
	call := message.ToolCall{ToolName: "edit_file", Args: map[string]interface{}{"path": "example.txt", "old_string": "before", "new_string": "after"}}

	result, err := executeToolCall(context.Background(), bufio.NewScanner(strings.NewReader("y\n")), io.Discard, tools, permission.Policy{}, call)
	if err != nil || !strings.HasPrefix(result, "Applied:") {
		t.Fatalf("result = %q, error = %v", result, err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "after\n" {
		t.Fatalf("file = %q, error = %v", data, err)
	}
}

func TestExecuteToolCallRequiresApprovalForCommands(t *testing.T) {
	root := t.TempDir()
	tools := tool.NewRegistry()
	if err := tools.Register(tool.NewRunCommand(root)); err != nil {
		t.Fatalf("register command tool: %v", err)
	}
	call := message.ToolCall{ToolName: "run_command", Args: map[string]interface{}{"program": "go", "args": []interface{}{"version"}}}
	var output bytes.Buffer

	result, err := executeToolCall(context.Background(), bufio.NewScanner(strings.NewReader("n\n")), &output, tools, permission.Policy{}, call)
	if err != nil || result != "Command was not approved." {
		t.Fatalf("result = %q, error = %v", result, err)
	}
	if !strings.Contains(output.String(), `Run command "go"?`) {
		t.Errorf("expected command approval prompt, got %q", output.String())
	}
}

func TestExecuteToolCallRunsApprovedCommand(t *testing.T) {
	root := t.TempDir()
	tools := tool.NewRegistry()
	if err := tools.Register(tool.NewRunCommand(root)); err != nil {
		t.Fatalf("register command tool: %v", err)
	}
	call := message.ToolCall{ToolName: "run_command", Args: map[string]interface{}{"program": "go", "args": []interface{}{"version"}}}

	result, err := executeToolCall(context.Background(), bufio.NewScanner(strings.NewReader("y\n")), io.Discard, tools, permission.Policy{}, call)
	if err != nil || !strings.HasPrefix(result, "go version ") {
		t.Fatalf("result = %q, error = %v", result, err)
	}
}

func TestToolCallGuardAllowsDifferentTools(t *testing.T) {
	guard := newToolCallGuard(5, 2)
	for _, toolName := range []string{"list_files", "search_text", "read_file", "edit_file", "read_file"} {
		if err := guard.Observe(toolName); err != nil {
			t.Fatalf("observe %q: %v", toolName, err)
		}
	}
}

func TestToolCallGuardStopsRepeatedTool(t *testing.T) {
	guard := newToolCallGuard(10, 2)
	_ = guard.Observe("edit_file")
	_ = guard.Observe("edit_file")
	if err := guard.Observe("edit_file"); err == nil || !strings.Contains(err.Error(), "3 times in a row") {
		t.Fatalf("expected repeated-tool error, got %v", err)
	}
}

func TestToolCallGuardStopsLongTurns(t *testing.T) {
	guard := newToolCallGuard(2, 10)
	_ = guard.Observe("list_files")
	_ = guard.Observe("search_text")
	if err := guard.Observe("read_file"); err == nil || !strings.Contains(err.Error(), "limit of 2") {
		t.Fatalf("expected call-limit error, got %v", err)
	}
}

func TestIsMutationTool(t *testing.T) {
	for _, toolName := range []string{"edit_file", "write_file", "run_command"} {
		if !isMutationTool(toolName) {
			t.Errorf("expected %s to be a mutation tool", toolName)
		}
	}
	if isMutationTool("read_file") {
		t.Error("read_file should not be a mutation tool")
	}
}

func TestStreamToMessageWritesBeforeStreamCompletes(t *testing.T) {
	allowFinish := make(chan struct{})
	stream := &controlledStream{allowFinish: allowFinish}
	output := &firstWriteBuffer{written: make(chan struct{})}
	done := make(chan error, 1)

	go func() {
		_, err := streamToMessage(stream, output)
		done <- err
	}()

	<-output.written
	if got := output.String(); got != "first" {
		t.Fatalf("expected first delta before stream completed, got %q", got)
	}

	close(allowFinish)
	if err := <-done; err != nil {
		t.Fatalf("stream message: %v", err)
	}
}

type controlledStream struct {
	allowFinish <-chan struct{}
	step        int
}

func (s *controlledStream) Recv() (provider.Delta, error) {
	s.step++
	switch s.step {
	case 1:
		return provider.Delta{Type: "text", Text: "first"}, nil
	case 2:
		<-s.allowFinish
		return provider.Delta{Type: "text", Text: " second"}, nil
	default:
		return provider.Delta{}, io.EOF
	}
}

func (s *controlledStream) Close() error {
	return nil
}

type firstWriteBuffer struct {
	bytes.Buffer
	written chan struct{}
	once    sync.Once
}

func (b *firstWriteBuffer) Write(data []byte) (int, error) {
	n, err := b.Buffer.Write(data)
	b.once.Do(func() { close(b.written) })
	return n, err
}
