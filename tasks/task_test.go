package tasks

import (
	"encoding/json"
	"testing"
)

// Task's JSON shape is a cross-surface contract: `taskloom list --json`
// marshals it directly, and the taskloom MCP tools emit the same snake_case
// keys (harp_id, text, status, ...). A scripted consumer must be able to
// treat both surfaces identically.
func TestTaskMarshalsSnakeCase(t *testing.T) {
	b, err := json.Marshal(Task{
		HarpID:        "swift-amber-falcon",
		Text:          "do the thing",
		Status:        StatusToDo,
		TextHash:      "abc123def456",
		Trigger:       "v2 ships",
		OriginSession: "zesty-slack-wager",
	})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"harp_id", "text", "status", "checked", "text_hash", "trigger", "origin_session"} {
		if _, ok := got[key]; !ok {
			t.Errorf("marshaled Task missing %q; keys: %v", key, got)
		}
	}
}

func TestTaskMarshalOmitsEmptyOptionalFields(t *testing.T) {
	b, err := json.Marshal(Task{HarpID: "old-dill", Text: "x", Status: StatusToDo})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"trigger", "origin_session"} {
		if _, ok := got[key]; ok {
			t.Errorf("empty %q should be omitted; keys: %v", key, got)
		}
	}
}

func TestSummaryMarshalsSnakeCase(t *testing.T) {
	b, err := json.Marshal(Summary{Counts: map[string]int{StatusToDo: 1}, InProgress: []string{"old-dill"}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["in_progress"]; !ok {
		t.Errorf("marshaled Summary missing in_progress; keys: %v", got)
	}
}
