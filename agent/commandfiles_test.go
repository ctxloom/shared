package agent

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSafeCommandRelPath pins the path-traversal guard the per-agent
// command/skill writers use for names and manifest lines: absolute paths and
// any ".." element are rejected; plain and nested relative names pass and
// join under dir.
func TestSafeCommandRelPath(t *testing.T) {
	dir := filepath.Join("/work", ".agents", "skills")

	tests := []struct {
		name string
		in   string
		ok   bool
		want string // joined path when ok
	}{
		{"plain name", "good.md", true, filepath.Join(dir, "good.md")},
		{"nested name", "group/cmd.md", true, filepath.Join(dir, "group", "cmd.md")},
		{"empty", "", false, ""},
		{"absolute", "/abs/path.md", false, ""},
		{"parent traversal", "../escape.md", false, ""},
		{"nested traversal", "a/../../b.md", false, ""},
		{"interior dotdot", "a/../b.md", false, ""},
		{"bare dotdot", "..", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := SafeCommandRelPath(dir, tt.in)
			assert.Equal(t, tt.ok, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
