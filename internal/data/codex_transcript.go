package data

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
)

type codexEntry struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type codexPayloadEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type codexPayloadResponseItem struct {
	Type      string `json:"type"`
	Role      string `json:"role"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Input     string `json:"input"`
	Status    string `json:"status"`
	Content   []struct {
		Type       string `json:"type"`
		Text       string `json:"text"`
		OutputText string `json:"output_text"`
		InputText  string `json:"input_text"`
	} `json:"content"`
}

type codexPayloadTokenCount struct {
	Info struct {
		LastTokenUsage struct {
			InputTokens       int64 `json:"input_tokens"`
			OutputTokens      int64 `json:"output_tokens"`
			CachedInputTokens int64 `json:"cached_input_tokens"`
		} `json:"last_token_usage"`
	} `json:"info"`
}

func LoadCodexTranscript(path string) (*Transcript, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	t := &Transcript{}
	roundIndex := 0
	currentRound := -1

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024)
	for scanner.Scan() {
		var entry codexEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}

		switch entry.Type {
		case "event_msg":
			var ev codexPayloadEvent
			if json.Unmarshal(entry.Payload, &ev) != nil {
				continue
			}
			switch ev.Type {
			case "user_message":
				isCtx := isSystemContext(ev.Message)
				role := "you"
				if isCtx {
					role = "context"
				}
				r := Round{
					Index:         roundIndex,
					UserTimestamp: entry.Timestamp,
					IsContext:     isCtx,
					Blocks:        []Block{{Role: role, Text: ev.Message}},
				}
				t.Rounds = append(t.Rounds, r)
				currentRound = len(t.Rounds) - 1
				roundIndex++
			case "agent_message":
				if currentRound < 0 {
					r := Round{Index: roundIndex}
					t.Rounds = append(t.Rounds, r)
					currentRound = len(t.Rounds) - 1
					roundIndex++
				}
				if ev.Message != "" {
					t.Rounds[currentRound].Blocks = append(t.Rounds[currentRound].Blocks, Block{
						Role: "claude",
						Text: ev.Message,
					})
				}
			case "token_count":
				if currentRound < 0 {
					continue
				}
				var tc codexPayloadTokenCount
				if json.Unmarshal(entry.Payload, &tc) != nil {
					continue
				}
				u := &t.Rounds[currentRound].Usage
				last := tc.Info.LastTokenUsage
				if last.InputTokens > u.InputTokens {
					u.InputTokens = last.InputTokens
				}
				u.OutputTokens += last.OutputTokens
				if last.CachedInputTokens > u.CacheRead {
					u.CacheRead = last.CachedInputTokens
				}
			}
		case "response_item":
			if currentRound < 0 {
				r := Round{Index: roundIndex}
				t.Rounds = append(t.Rounds, r)
				currentRound = len(t.Rounds) - 1
				roundIndex++
			}
			var ri codexPayloadResponseItem
			if json.Unmarshal(entry.Payload, &ri) != nil {
				continue
			}
			switch ri.Type {
			case "function_call":
				addCodexToolCall(&t.Rounds[currentRound], ri.Name, ri.Arguments)
			case "custom_tool_call":
				addCodexToolCall(&t.Rounds[currentRound], ri.Name, ri.Input)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return t, nil
}

func addCodexToolCall(r *Round, name, rawInput string) {
	inputJSON := normalizeRawJSON(rawInput)
	tc := ToolCall{
		Name:         name,
		InputSummary: codexToolInputSummary(name, rawInput),
		InputJSON:    inputJSON,
	}
	r.Blocks = append(r.Blocks, Block{Role: "tool", ToolCall: &tc})
}

func normalizeRawJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var v interface{}
	if json.Unmarshal([]byte(raw), &v) != nil {
		return raw
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}
	return string(b)
}

func codexToolInputSummary(toolName, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	var m map[string]interface{}
	if json.Unmarshal([]byte(raw), &m) != nil {
		const maxSummary = 100
		if len(raw) <= maxSummary {
			return raw
		}
		return raw[:maxSummary-3] + "..."
	}
	switch toolName {
	case "exec_command":
		if cmd, ok := m["cmd"].(string); ok {
			return cmd
		}
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// deterministic output
	if len(keys) == 0 {
		return ""
	}
	for _, k := range []string{"path", "cmd", "ref_id", "q"} {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	for _, k := range keys {
		if v, ok := m[k].(string); ok && v != "" {
			return v
		}
	}
	return raw
}
