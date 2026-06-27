// Package clidiag is the ctxloom family's stderr diagnostic convention:
// fault-tolerant warnings prefixed "<prog>: warning:". Per the fault-tolerance
// philosophy, components warn and continue rather than crash, so this is the
// one place that owns the prefix format. The prog is a parameter (not hardcoded
// to "ctxloom") so every binary stamps its own name.
package clidiag

import (
	"fmt"
	"io"
	"os"
)

// Warn prints a "<prog>: warning: <msg>" line to stderr.
func Warn(prog, format string, args ...any) {
	Fwarn(os.Stderr, prog, format, args...)
}

// Fwarn writes a "<prog>: warning: <msg>" line to w (the testable form).
func Fwarn(w io.Writer, prog, format string, args ...any) {
	fmt.Fprintf(w, prog+": warning: "+format+"\n", args...)
}

// Warner binds a program name so callers that warn repeatedly don't repeat it.
//
//	warn := clidiag.Warner("taskloom")
//	warn.Warn("sync failed: %v", err)
type Warner string

// Warn prints a "<prog>: warning: <msg>" line to stderr for the bound prog.
func (p Warner) Warn(format string, args ...any) {
	Warn(string(p), format, args...)
}
