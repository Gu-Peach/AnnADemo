package executa

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestInitializeDeclaresSamplingCapability(t *testing.T) {
	out := runStatic(t, "{\"jsonrpc\":\"2.0\",\"method\":\"initialize\",\"id\":1,\"params\":{\"protocolVersion\":\"2.0\"}}\n")
	var resp map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &resp); err != nil {
		t.Fatalf("json: %v", err)
	}
	result := resp["result"].(map[string]any)
	caps := result["client_capabilities"].(map[string]any)
	if _, ok := caps["sampling"]; !ok {
		t.Fatalf("sampling capability missing: %#v", caps)
	}
}

func TestDescribeManifestUsesAnnaParameters(t *testing.T) {
	manifest := Manifest()
	if manifest["host_capabilities"].([]string)[0] != "llm.sample" {
		t.Fatalf("missing host capability")
	}
	tools := manifest["tools"].([]map[string]any)
	params := tools[0]["parameters"].([]map[string]any)
	if len(params) == 0 || params[0]["name"] != "notes" {
		t.Fatalf("bad params: %#v", params)
	}
	if _, exists := tools[0]["input_schema"]; exists {
		t.Fatalf("manifest should not use MCP input_schema")
	}
}

func TestUnknownMethod(t *testing.T) {
	out := runStatic(t, "{\"jsonrpc\":\"2.0\",\"method\":\"missing\",\"id\":9}\n")
	if !strings.Contains(out, "-32601") {
		t.Fatalf("expected method error, got %s", out)
	}
}

func TestEmptyNotesDoesNotSample(t *testing.T) {
	line := `{"jsonrpc":"2.0","method":"invoke","id":2,"params":{"tool":"summarize","arguments":{"notes":[]}}}` + "\n"
	out := runStatic(t, line)
	if strings.Contains(out, "sampling/createMessage") {
		t.Fatalf("empty notes should not sample: %s", out)
	}
	if !strings.Contains(out, "at least one note is required") {
		t.Fatalf("expected no-notes error: %s", out)
	}
}

func TestSamplingRoundTrip(t *testing.T) {
	inputReader, inputWriter := io.Pipe()
	var out strings.Builder
	server := NewServer(inputReader, &out, &strings.Builder{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- server.Run(ctx) }()

	_, _ = inputWriter.Write([]byte(`{"jsonrpc":"2.0","method":"invoke","id":2,"params":{"tool":"summarize","invoke_id":"inv-1","arguments":{"notes":[{"order":1,"content":"Follow up with customer"}],"max_words":40}}}` + "\n"))

	waitFor(t, func() bool { return strings.Contains(out.String(), "sampling/createMessage") })
	firstLine := strings.Split(strings.TrimSpace(out.String()), "\n")[0]
	var samplingReq map[string]any
	if err := json.Unmarshal([]byte(firstLine), &samplingReq); err != nil {
		t.Fatalf("sampling json: %v line=%s", err, firstLine)
	}
	id := samplingReq["id"].(string)
	params := samplingReq["params"].(map[string]any)
	metadata := params["metadata"].(map[string]any)
	if metadata["executa_invoke_id"] != "inv-1" {
		t.Fatalf("missing invoke metadata: %#v", metadata)
	}
	response := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]any{"content": map[string]any{"type": "text", "text": "Customer follow-up is the priority."}, "model": "mock-model"},
	}
	encoded, _ := json.Marshal(response)
	_, _ = inputWriter.Write(append(encoded, '\n'))
	waitFor(t, func() bool { return strings.Contains(out.String(), "Customer follow-up is the priority") })
	_ = inputWriter.Close()
	if err := <-done; err != nil {
		t.Fatalf("server run: %v", err)
	}
}

func runStatic(t *testing.T, input string) string {
	t.Helper()
	in := strings.NewReader(input)
	var out strings.Builder
	server := NewServer(in, &out, &strings.Builder{})
	if err := server.Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	waitFor(t, func() bool { return strings.TrimSpace(out.String()) != "" })
	return out.String()
}

func waitFor(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if check() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met")
}
