package executa

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const ProtocolV2 = "2.0"

var ErrNoNotes = errors.New("at least one note is required")

type RawMessage = json.RawMessage

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type Note struct {
	ID        string `json:"id,omitempty"`
	Order     int    `json:"order"`
	Content   string `json:"content"`
	CreatedAt string `json:"createdAt,omitempty"`
}

type InvokeParams struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
	InvokeID  string          `json:"invoke_id,omitempty"`
}

type SummarizeArgs struct {
	Notes    []Note `json:"notes"`
	MaxWords int    `json:"max_words,omitempty"`
}

type SamplingResult struct {
	Content    any    `json:"content"`
	Model      string `json:"model,omitempty"`
	Usage      any    `json:"usage,omitempty"`
	StopReason string `json:"stopReason,omitempty"`
}

type SamplingContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type pendingCall struct {
	result chan Response
}

type Server struct {
	reader  *bufio.Scanner
	writer  *bufio.Writer
	logger  *log.Logger
	pending map[string]pendingCall
	mu      sync.Mutex
	seq     atomic.Int64
}

func NewServer(in io.Reader, out io.Writer, errOut io.Writer) *Server {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	return &Server{
		reader:  scanner,
		writer:  bufio.NewWriter(out),
		logger:  log.New(errOut, "mini-notes-summarizer: ", log.LstdFlags),
		pending: map[string]pendingCall{},
	}
}

func (s *Server) Run(ctx context.Context) error {
	for s.reader.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		line := strings.TrimSpace(s.reader.Text())
		if line == "" {
			continue
		}
		if err := s.handleLine(ctx, []byte(line)); err != nil {
			s.logger.Printf("handler error: %v", err)
		}
	}
	return s.reader.Err()
}

func (s *Server) handleLine(ctx context.Context, line []byte) error {
	var req Request
	if err := json.Unmarshal(line, &req); err != nil {
		return s.writeResponse(Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &RPCError{Code: -32700, Message: "Parse error"}})
	}
	if req.Method == "" {
		return s.dispatchReverseResponse(req)
	}

	resp := Response{JSONRPC: "2.0", ID: req.ID}
	switch req.Method {
	case "initialize":
		resp.Result = s.initialize(req.Params)
	case "describe":
		resp.Result = Manifest()
	case "health":
		resp.Result = map[string]any{"status": "healthy", "version": "0.1.0"}
	case "shutdown":
		resp.Result = map[string]any{"ok": true}
	case "invoke":
		go s.handleInvoke(ctx, req)
		return nil
	default:
		resp.Error = &RPCError{Code: -32601, Message: "Method not found: " + req.Method}
	}
	return s.writeResponse(resp)
}

func (s *Server) handleInvoke(ctx context.Context, req Request) {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	result, rpcErr := s.invoke(ctx, req.Params)
	if rpcErr != nil {
		resp.Error = rpcErr
	} else {
		resp.Result = result
	}
	if err := s.writeResponse(resp); err != nil {
		s.logger.Printf("write invoke response failed: %v", err)
	}
}

func (s *Server) initialize(params json.RawMessage) map[string]any {
	protocol := "1.1"
	if len(params) > 0 {
		var decoded struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(params, &decoded)
		if decoded.ProtocolVersion != "" {
			protocol = decoded.ProtocolVersion
		}
	}
	capabilities := map[string]any{}
	if protocol == ProtocolV2 {
		capabilities["sampling"] = map[string]any{}
	} else {
		s.logger.Printf("host did not negotiate protocol 2.0; sampling will fail if invoked")
	}
	return map[string]any{
		"protocolVersion":     protocol,
		"serverInfo":          map[string]string{"name": "Mini Notes Summarizer", "version": "0.1.0"},
		"client_capabilities": capabilities,
		"capabilities":        map[string]any{},
	}
}

func Manifest() map[string]any {
	return map[string]any{
		"name":              "mini-notes-summarizer",
		"display_name":      "Mini Notes Summarizer",
		"version":           "0.1.0",
		"description":       "Summarizes Mini Notes App notes through Anna host LLM sampling.",
		"host_capabilities": []string{"llm.sample"},
		"tools": []map[string]any{
			{
				"name":        "summarize",
				"description": "Summarize the current Mini Notes list using the Anna host LLM.",
				"timeout":     60,
				"streaming":   false,
				"parameters": []map[string]any{
					{"name": "notes", "type": "array", "description": "Notes to summarize, each with order and content.", "required": true, "items": map[string]string{"type": "object"}},
					{"name": "max_words", "type": "integer", "description": "Approximate maximum words for the summary.", "required": false, "default": 80},
				},
			},
		},
		"runtime": map[string]any{"type": "binary", "min_version": "0.1.0"},
	}
}

func (s *Server) invoke(ctx context.Context, params json.RawMessage) (any, *RPCError) {
	var decoded InvokeParams
	if err := json.Unmarshal(params, &decoded); err != nil {
		return nil, &RPCError{Code: -32602, Message: "Invalid invoke params: " + err.Error()}
	}
	if decoded.Tool != "summarize" {
		return nil, &RPCError{Code: -32601, Message: "Unknown tool: " + decoded.Tool}
	}
	var args SummarizeArgs
	if len(decoded.Arguments) > 0 {
		if err := json.Unmarshal(decoded.Arguments, &args); err != nil {
			return nil, &RPCError{Code: -32602, Message: "Invalid summarize arguments: " + err.Error()}
		}
	}
	started := time.Now()
	result, err := s.summarize(ctx, args, decoded.InvokeID)
	if err != nil {
		return map[string]any{"success": false, "error": err.Error(), "duration_ms": time.Since(started).Milliseconds()}, nil
	}
	return map[string]any{"success": true, "data": result, "duration_ms": time.Since(started).Milliseconds()}, nil
}

func (s *Server) summarize(ctx context.Context, args SummarizeArgs, invokeID string) (map[string]any, error) {
	notes := make([]Note, 0, len(args.Notes))
	for _, note := range args.Notes {
		content := strings.TrimSpace(note.Content)
		if content == "" {
			continue
		}
		note.Content = content
		notes = append(notes, note)
	}
	if len(notes) == 0 {
		return nil, ErrNoNotes
	}
	maxWords := args.MaxWords
	if maxWords <= 0 {
		maxWords = 80
	}
	if maxWords < 20 {
		maxWords = 20
	}
	if maxWords > 400 {
		maxWords = 400
	}

	result, err := s.createSamplingMessage(ctx, notes, maxWords, invokeID)
	if err != nil {
		return nil, err
	}
	summary := extractTextContent(result.Content)
	if summary == "" {
		return nil, errors.New("sampling response did not contain text content")
	}
	return map[string]any{"summary": summary, "model": result.Model, "usage": result.Usage, "stopReason": result.StopReason}, nil
}

func extractTextContent(content any) string {
	switch value := content.(type) {
	case string:
		return strings.TrimSpace(value)
	case map[string]any:
		if text, ok := value["text"].(string); ok {
			return strings.TrimSpace(text)
		}
		if nested, ok := value["content"]; ok {
			return extractTextContent(nested)
		}
	case []any:
		for _, item := range value {
			if text := extractTextContent(item); text != "" {
				return text
			}
		}
	case SamplingContent:
		return strings.TrimSpace(value.Text)
	}
	return ""
}

func (s *Server) createSamplingMessage(ctx context.Context, notes []Note, maxWords int, invokeID string) (SamplingResult, error) {
	id := fmt.Sprintf("sampling-%d", s.seq.Add(1))
	prompt := buildPrompt(notes, maxWords)
	params := map[string]any{
		"messages": []map[string]any{
			{"role": "user", "content": map[string]any{"type": "text", "text": prompt}},
		},
		"maxTokens":    maxWords * 5,
		"systemPrompt": "You summarize short personal notes clearly and concisely. Return only the summary, no preamble.",
		"metadata": map[string]any{
			"executa_invoke_id": invokeID,
			"tool":              "summarize",
			"note_count":        len(notes),
		},
		"timeoutMs": 60000,
	}
	request := map[string]any{"jsonrpc": "2.0", "id": id, "method": "sampling/createMessage", "params": params}

	resultCh := make(chan Response, 1)
	s.mu.Lock()
	s.pending[id] = pendingCall{result: resultCh}
	s.mu.Unlock()

	if err := s.writeJSON(request); err != nil {
		s.removePending(id)
		return SamplingResult{}, err
	}

	select {
	case <-ctx.Done():
		s.removePending(id)
		return SamplingResult{}, ctx.Err()
	case response := <-resultCh:
		if response.Error != nil {
			return SamplingResult{}, fmt.Errorf("sampling/createMessage failed: [%d] %s", response.Error.Code, response.Error.Message)
		}
		payload, err := json.Marshal(response.Result)
		if err != nil {
			return SamplingResult{}, err
		}
		var result SamplingResult
		if err := json.Unmarshal(payload, &result); err != nil {
			return SamplingResult{}, err
		}
		return result, nil
	case <-time.After(65 * time.Second):
		s.removePending(id)
		return SamplingResult{}, errors.New("sampling/createMessage timed out")
	}
}

func buildPrompt(notes []Note, maxWords int) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Summarize these Mini Notes in at most %d words. Preserve important actions, deadlines, and themes.\n\nNotes:\n", maxWords)
	for _, note := range notes {
		order := note.Order
		if order <= 0 {
			order = 1
		}
		fmt.Fprintf(&builder, "%d. %s\n", order, note.Content)
	}
	return builder.String()
}

func (s *Server) dispatchReverseResponse(msg Request) error {
	id := strings.Trim(string(msg.ID), "\"")
	if id == "" || id == "null" {
		return nil
	}
	s.mu.Lock()
	call, ok := s.pending[id]
	if ok {
		delete(s.pending, id)
	}
	s.mu.Unlock()
	if !ok {
		s.logger.Printf("unmatched response id=%s", id)
		return nil
	}
	var result any
	if len(msg.Result) > 0 {
		if err := json.Unmarshal(msg.Result, &result); err != nil {
			call.result <- Response{JSONRPC: msg.JSONRPC, ID: msg.ID, Error: &RPCError{Code: -32700, Message: "Invalid reverse response result: " + err.Error()}}
			return nil
		}
	}
	call.result <- Response{JSONRPC: msg.JSONRPC, ID: msg.ID, Result: result, Error: msg.Error}
	return nil
}

func (s *Server) removePending(id string) {
	s.mu.Lock()
	delete(s.pending, id)
	s.mu.Unlock()
}

func (s *Server) writeResponse(resp Response) error {
	return s.writeJSON(resp)
}

func (s *Server) writeJSON(value any) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.NewEncoder(s.writer).Encode(value); err != nil {
		return err
	}
	return s.writer.Flush()
}
