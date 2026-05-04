// Package bootstrap detects and offers to install dots' two runtime
// prerequisites — Nix and the workspace clone — before deploy. Every
// state-mutating action is gated by an explicit Y/N consent prompt that
// echoes the EXACT command being offered, in keeping with the Honesty
// priority. ADR-0010 records the contract.
//
// Imports are deliberately limited to os/exec, os, and io plumbing.
// Per ADR-0009, subprocess invocation of curl/sh/git is not an I1
// violation; the bootstrap package contains no Nix-related Go imports.
package bootstrap

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

// Confirm prints question, the exact command on its own indented line,
// then "[y/N]" and reads a single response from in. The default is No
// — anything other than "y" / "yes" (case-insensitive) declines.
//
// Showing the command before consent is the contract: the user sees
// what runs before they approve. No silent execution.
func Confirm(out io.Writer, in io.Reader, question, command string) (bool, error) {
	if _, err := fmt.Fprintf(out, "%s\n\n  $ %s\n\nContinue? [y/N] ", question, command); err != nil {
		return false, err
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && err != io.EOF {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
