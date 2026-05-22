package process

import (
	"strings"
	"testing"
)

func TestParseOutput_Basic(t *testing.T) {
	input := `UID        PID  PPID  C STIME TTY          TIME CMD
root         1     0  0  2024 ?        00:01:23 /usr/lib/systemd/systemd --system
root      1255     1  0  2024 ?        01:23:45 /usr/bin/sysprobe
root    116865     1  0  2024 ?        00:10:34 sshd: /usr/sbin/sshd -D
root    155669 116865  0 14:20 ?        00:00:00 sshd-session
root    1949     1  0  2024 ?        00:05:12 ./titanagent
`
	tree, errs := ParseOutput(strings.NewReader(input))
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	if len(tree) != 5 {
		t.Fatalf("expected 5 entries, got %d", len(tree))
	}

	// Check sshd parent
	if tree[116865].PPID != 1 {
		t.Errorf("sshd PPID: expected 1, got %d", tree[116865].PPID)
	}
	if tree[116865].Name != "sshd" {
		t.Errorf("sshd Name: expected sshd, got %s", tree[116865].Name)
	}

	// Check sshd-session child
	if tree[155669].PPID != 116865 {
		t.Errorf("sshd-session PPID: expected 116865, got %d", tree[155669].PPID)
	}
	if tree[155669].Name != "sshd-session" {
		t.Errorf("sshd-session Name: expected sshd-session, got %s", tree[155669].Name)
	}

	// Check sysprobe (basename extraction)
	if tree[1255].Name != "sysprobe" {
		t.Errorf("sysprobe Name: expected sysprobe, got %s", tree[1255].Name)
	}

	// Check ./titanagent (relative path)
	if tree[1949].Name != "titanagent" {
		t.Errorf("titanagent Name: expected titanagent, got %s", tree[1949].Name)
	}
}

func TestParseOutput_SkipHeader(t *testing.T) {
	input := `UID        PID  PPID  C STIME TTY          TIME CMD
root      100     1  0  2024 ?        00:00:00 /bin/test
`
	tree, _ := ParseOutput(strings.NewReader(input))
	if len(tree) != 1 {
		t.Fatalf("expected 1 entry (header skipped), got %d", len(tree))
	}
}

func TestParseOutput_TooFewFields(t *testing.T) {
	input := `UID        PID  PPID  C STIME TTY          TIME CMD
root 100
`
	_, errs := ParseOutput(strings.NewReader(input))
	if len(errs) == 0 {
		t.Fatal("expected error for too few fields")
	}
}

func TestCleanName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"sshd:", "sshd"},
		{"/usr/sbin/sshd", "sshd"},
		{"./titanagent", "titanagent"},
		{"/usr/bin/sysprobe", "sysprobe"},
		{"nginx", "nginx"},
		{"docker-proxy", "docker-proxy"},
	}

	for _, tt := range tests {
		got := cleanName(tt.input)
		if got != tt.expected {
			t.Errorf("cleanName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
