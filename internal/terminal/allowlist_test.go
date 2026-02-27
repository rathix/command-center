package terminal

import (
	"strings"
	"testing"
)

func TestAllowlist_Validate(t *testing.T) {
	al := NewAllowlist([]string{"kubectl", "talosctl", "helm"})

	tests := []struct {
		name    string
		input   string
		wantCmd string
		wantErr string
	}{
		{
			name:    "allowed command",
			input:   "kubectl get pods",
			wantCmd: "kubectl",
		},
		{
			name:    "allowed command with no args",
			input:   "helm",
			wantCmd: "helm",
		},
		{
			name:    "path-based command extracts basename",
			input:   "/usr/bin/kubectl get pods",
			wantCmd: "kubectl",
		},
		{
			name:    "disallowed command",
			input:   "rm -rf /",
			wantErr: `command "rm" is not allowed`,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: "empty command",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: "empty command",
		},
		{
			name:    "pipe rejected",
			input:   "kubectl get pods | grep foo",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "and chain rejected",
			input:   "kubectl get pods && kubectl delete pod foo",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "semicolon rejected",
			input:   "kubectl get pods; rm -rf /",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "backtick rejected",
			input:   "kubectl get `whoami`",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "command substitution rejected",
			input:   "kubectl get $(whoami)",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "redirect rejected",
			input:   "kubectl get pods > /tmp/out",
			wantErr: "shell operators are not allowed",
		},
		{
			name:    "or chain rejected",
			input:   "kubectl get pods || true",
			wantErr: "shell operators are not allowed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, _, err := al.Validate(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cmd != tt.wantCmd {
				t.Errorf("expected command %q, got %q", tt.wantCmd, cmd)
			}
		})
	}
}

func TestAllowlist_ValidateArgs(t *testing.T) {
	al := NewAllowlist([]string{"kubectl"})
	_, args, err := al.Validate("kubectl get pods -n kube-system")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}
	expected := []string{"get", "pods", "-n", "kube-system"}
	for i, a := range args {
		if a != expected[i] {
			t.Errorf("arg[%d]: expected %q, got %q", i, expected[i], a)
		}
	}
}

func TestAllowlist_ErrorMessageListsAllowed(t *testing.T) {
	al := NewAllowlist([]string{"kubectl", "talosctl", "helm"})
	_, _, err := al.Validate("bash")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, cmd := range []string{"kubectl", "talosctl", "helm"} {
		if !strings.Contains(err.Error(), cmd) {
			t.Errorf("error message should contain %q: %s", cmd, err.Error())
		}
	}
}
