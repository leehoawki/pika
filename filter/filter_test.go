package filter

import (
	"testing"

	"github.com/leehoawki/pika/model"
)

func sampleConns() []model.Connection {
	return []model.Connection{
		{LocalPort: 80, ProcessName: "nginx", PID: 1234, State: "ESTABLISHED", RemoteAddr: "10.0.0.1"},
		{LocalPort: 443, ProcessName: "nginx", PID: 1234, State: "ESTABLISHED", RemoteAddr: "10.0.0.2"},
		{LocalPort: 22, ProcessName: "sshd", PID: 567, State: "LISTEN", RemoteAddr: "0.0.0.0"},
		{LocalPort: 3306, ProcessName: "mysqld", PID: 890, State: "ESTABLISHED", RemoteAddr: "127.0.0.1"},
	}
}

func TestApply_NoFilter(t *testing.T) {
	conns := sampleConns()
	result := Apply(conns, Opts{})
	if len(result) != len(conns) {
		t.Fatalf("no filter: expected %d, got %d", len(conns), len(result))
	}
}

func TestApply_FilterByProcess(t *testing.T) {
	conns := sampleConns()
	result := Apply(conns, Opts{Process: "nginx"})
	if len(result) != 2 {
		t.Fatalf("nginx: expected 2, got %d", len(result))
	}
}

func TestApply_FilterByProcess_Substring(t *testing.T) {
	conns := sampleConns()
	result := Apply(conns, Opts{Process: "sql"})
	if len(result) != 1 {
		t.Fatalf("substring 'sql': expected 1, got %d", len(result))
	}
	if result[0].ProcessName != "mysqld" {
		t.Errorf("expected mysqld, got %s", result[0].ProcessName)
	}
}

func TestApply_NoMatch(t *testing.T) {
	conns := sampleConns()
	result := Apply(conns, Opts{Process: "notexist"})
	if len(result) != 0 {
		t.Fatalf("expected 0, got %d", len(result))
	}
}
