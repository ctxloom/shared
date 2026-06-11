package agent

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/afero"
)

// CommandExport is the agent-agnostic slash-command export spec for one prompt.
// It is the abstraction the per-agent command writers (claude, antigravity) consume,
// so they never import ctxloom's bundle types: ctxloom maps each
// bundles.LoadedContent to a CommandExport for the target agent (resolving that
// agent's enablement + metadata) at the wiring boundary. Fields beyond
// Name/Content/Description are slash-command frontmatter; agents that don't use
// a given field simply ignore it.
type CommandExport struct {
	Name         string   // Full name (bundle/item); path separators allowed
	Content      string   // The command body
	Enabled      bool     // Resolved enablement for the target agent
	Description  string   // For /help display
	ArgumentHint string   // Autocomplete hint (unused by antigravity)
	AllowedTools []string // Tool restrictions (unused by antigravity)
	Model        string   // Override model (unused by antigravity)
}

// CommandFileOption configures command file writing.
type CommandFileOption func(*commandFileOptions)

type commandFileOptions struct {
	fs afero.Fs
}

// WithCommandFS sets the filesystem for command file operations.
func WithCommandFS(fs afero.Fs) CommandFileOption {
	return func(o *commandFileOptions) {
		o.fs = fs
	}
}

// SafeCommandRelPath validates name as a relative path confined to dir and
// returns the cleaned joined path. Command/skill names and manifest lines can
// originate in bundle content (potentially remote), so the per-agent writers
// must never join them into their managed directory blindly: a "../x" name
// escapes the tree on write, and a malicious manifest line deletes files
// outside the tree on cleanup. Rejected (ok == false): empty names, absolute
// paths, any ".." path element, and any join whose result escapes dir.
// Subdirectory names without traversal ("group/cmd") pass.
func SafeCommandRelPath(dir, name string) (string, bool) {
	if name == "" || filepath.IsAbs(name) || filepath.IsAbs(filepath.FromSlash(name)) {
		return "", false
	}
	for part := range strings.SplitSeq(filepath.ToSlash(name), "/") {
		if part == ".." {
			return "", false
		}
	}
	joined := filepath.Join(dir, filepath.FromSlash(name))
	// Belt and braces: verify the cleaned join really stays under dir.
	rel, err := filepath.Rel(dir, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return joined, true
}

// ResolveCommandFS applies the options and returns the filesystem to use,
// defaulting to the OS filesystem. Per-agent command writers (in the claude and
// antigravity packages) call this so they can honor WithCommandFS without reaching
// the unexported option struct.
func ResolveCommandFS(opts ...CommandFileOption) afero.Fs {
	options := &commandFileOptions{fs: afero.NewOsFs()}
	for _, opt := range opts {
		opt(options)
	}
	return options.fs
}

// ManagedWriteOption configures WriteManagedCommandFiles.
type ManagedWriteOption func(*managedWriteOptions)

type managedWriteOptions struct {
	manifestTrailingNewline bool
}

// WithManifestTrailingNewline makes the manifest end with a trailing newline
// (the antigravity writer's historical byte shape). The default (no trailing
// newline) matches the claude/codex writers; both parse identically, the
// option only preserves each agent's existing on-disk bytes.
func WithManifestTrailingNewline() ManagedWriteOption {
	return func(o *managedWriteOptions) { o.manifestTrailingNewline = true }
}

// WriteManagedCommandFiles is the manifest-scoped slash-command/skill file
// writer shared by the per-agent command writers (claude, codex, antigravity).
// dir is shared territory with user-authored files, so it is never wiped
// wholesale: ctxloom tracks the files it wrote in a manifest (dir/manifestName)
// and removes exactly that set before writing the current one, so the written
// set always mirrors the enabled exports.
//
// render maps one enabled export to its file: the path relative to dir plus
// the file content. Both command names and manifest lines originate in bundle
// content (potentially remote), so every name and rendered path is validated
// with SafeCommandRelPath — traversal/absolute names are skipped with a
// warning on write and never followed on cleanup.
//
// dir itself is only created when at least one file is written, and the
// manifest is only (re)written when at least one file was written; with
// nothing to write the previous manifest-tracked set and manifest are simply
// removed.
func WriteManagedCommandFiles(fs afero.Fs, dir, manifestName string, cmds []CommandExport, render func(CommandExport) (relPath string, content []byte, err error), opts ...ManagedWriteOption) error {
	o := &managedWriteOptions{}
	for _, opt := range opts {
		opt(o)
	}
	manifestPath := filepath.Join(dir, manifestName)

	// Remove the previous ctxloom-written set (manifest-tracked only).
	// Manifest lines are data, not trusted paths: a doctored line ("../x",
	// absolute) must not delete outside the managed tree.
	if data, err := afero.ReadFile(fs, manifestPath); err == nil {
		for _, name := range strings.Split(string(data), "\n") {
			if name = strings.TrimSpace(name); name != "" {
				path, ok := SafeCommandRelPath(dir, name)
				if !ok {
					Warn("skipping unsafe command manifest entry %q: not a relative path inside %s", name, dir)
					continue
				}
				_ = fs.Remove(path)
			}
		}
		_ = fs.Remove(manifestPath)
	}

	var written []string
	for _, c := range cmds {
		if !c.Enabled {
			continue
		}
		// Reject absolute/traversal names outright before any path is derived
		// from them. Nested names without traversal ("group/cmd") remain
		// allowed; how they map to a path is the renderer's choice.
		if _, ok := SafeCommandRelPath(dir, c.Name); !ok {
			Warn("skipping command %q: name is not a relative path inside %s", c.Name, dir)
			continue
		}
		relPath, content, err := render(c)
		if err != nil {
			return fmt.Errorf("render command %s: %w", c.Name, err)
		}
		path, ok := SafeCommandRelPath(dir, relPath)
		if !ok {
			Warn("skipping command %q: rendered path %q is not a relative path inside %s", c.Name, relPath, dir)
			continue
		}
		if len(written) == 0 {
			if err := fs.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create command dir: %w", err)
			}
		}
		if parent := filepath.Dir(path); parent != filepath.Clean(dir) {
			if err := fs.MkdirAll(parent, 0755); err != nil {
				return fmt.Errorf("create command subdir %s: %w", parent, err)
			}
		}
		if err := afero.WriteFile(fs, path, content, 0644); err != nil {
			return fmt.Errorf("write command %s: %w", c.Name, err)
		}
		written = append(written, relPath)
	}

	if len(written) == 0 {
		return nil
	}
	manifest := strings.Join(written, "\n")
	if o.manifestTrailingNewline {
		manifest += "\n"
	}
	return afero.WriteFile(fs, manifestPath, []byte(manifest), 0644)
}

// mustacheVarRe matches {{variable}} placeholders in command bodies.
var mustacheVarRe = regexp.MustCompile(`\{\{(\w+)\}\}`)

// TransformMustacheToPositional replaces {{variable}} patterns with $1, $2,
// etc. Variables are assigned positions by first occurrence order. This is the
// argument transform shared by the claude and codex command renderers (both
// CLIs use positional $N prompt arguments).
func TransformMustacheToPositional(content string) string {
	varNum := 1
	seen := make(map[string]int)

	return mustacheVarRe.ReplaceAllStringFunc(content, func(match string) string {
		varName := mustacheVarRe.FindStringSubmatch(match)[1]
		if num, exists := seen[varName]; exists {
			return fmt.Sprintf("$%d", num)
		}
		seen[varName] = varNum
		num := varNum
		varNum++
		return fmt.Sprintf("$%d", num)
	})
}

// EscapeYAMLString quotes a string for safe inclusion in YAML frontmatter when
// it contains special characters.
func EscapeYAMLString(s string) string {
	needsQuotes := strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") ||
		strings.HasPrefix(s, " ") ||
		strings.HasSuffix(s, " ") ||
		strings.Contains(s, "\n")
	if needsQuotes {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}
