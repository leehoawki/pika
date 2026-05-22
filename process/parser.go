package process

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// Info holds basic process info from ps -ef.
type Info struct {
	PID  int
	PPID int
	Name string
}

// Tree maps PID to process Info.
type Tree map[int]Info

// Parse executes ps -ef and returns a process tree.
// Errors are non-fatal — the tool works without ps data.
func Parse() (Tree, []error) {
	cmd := exec.Command("ps", "-ef")
	output, err := cmd.Output()
	if err != nil {
		return nil, []error{fmt.Errorf("ps -ef failed: %w", err)}
	}
	return ParseOutput(strings.NewReader(string(output)))
}

// ParseOutput parses ps -ef output into a process tree.
func ParseOutput(r io.Reader) (Tree, []error) {
	tree := make(Tree)
	var errs []error
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if lineNum == 1 || line == "" {
			continue
		}
		info, err := parseLine(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}
		tree[info.PID] = info
	}

	return tree, errs
}

// ps -ef fields: UID PID PPID C STIME TTY TIME CMD [args...]
func parseLine(line string) (Info, error) {
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return Info{}, fmt.Errorf("too few fields: %q", line)
	}

	pid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Info{}, fmt.Errorf("invalid PID: %q", fields[1])
	}
	ppid, err := strconv.Atoi(fields[2])
	if err != nil {
		return Info{}, fmt.Errorf("invalid PPID: %q", fields[2])
	}

	name := cleanName(fields[7])
	return Info{PID: pid, PPID: ppid, Name: name}, nil
}

// cleanName extracts a clean process name from a command path.
// "sshd:" → "sshd", "/usr/sbin/sshd" → "sshd", "./titanagent" → "titanagent"
func cleanName(cmd string) string {
	name := filepath.Base(cmd)
	name = strings.TrimRight(name, ":")
	return name
}
