package terminal

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Allowlist validates commands against a configured set of allowed binaries.
type Allowlist struct {
	commands map[string]struct{}
	list     []string // for error messages
}

// NewAllowlist creates an Allowlist from the given command names.
func NewAllowlist(commands []string) *Allowlist {
	m := make(map[string]struct{}, len(commands))
	for _, cmd := range commands {
		m[cmd] = struct{}{}
	}
	return &Allowlist{commands: m, list: commands}
}

// Validate checks if the given command line is allowed.
// It extracts the first token (the command), strips any path prefix,
// and checks against the allowlist. Pipes and chains are rejected.
func (a *Allowlist) Validate(input string) (command string, args []string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", nil, fmt.Errorf("empty command")
	}

	// Reject pipes and chains
	for _, ch := range []string{"|", "&&", "||", ";", "`", "$(", ">"} {
		if strings.Contains(input, ch) {
			return "", nil, fmt.Errorf("shell operators are not allowed")
		}
	}

	parts := strings.Fields(input)
	cmd := parts[0]

	// Extract basename if path is provided (e.g., /usr/bin/kubectl -> kubectl)
	cmd = filepath.Base(cmd)

	if _, ok := a.commands[cmd]; !ok {
		return "", nil, fmt.Errorf("command %q is not allowed. Allowed: %s", cmd, strings.Join(a.list, ", "))
	}

	return cmd, parts[1:], nil
}
