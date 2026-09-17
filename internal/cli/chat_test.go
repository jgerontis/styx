package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/jgerontis/styx/internal/provider"
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
