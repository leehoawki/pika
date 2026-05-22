package docker

import (
	"strings"
	"testing"
)

func TestParseOutput(t *testing.T) {
	input := `nginx-web	0.0.0.0:9997->9997/tcp, :::9997->9997/tcp
redis-cache	0.0.0.0:6379->6379/tcp
app-server	0.0.0.0:8080->8080/tcp, 0.0.0.0:8443->443/tcp
`
	pm, _ := ParseOutput(strings.NewReader(input))

	if len(pm) != 4 {
		t.Fatalf("expected 4 port mappings, got %d", len(pm))
	}
	if pm[9997] != "nginx-web" {
		t.Errorf("port 9997: expected nginx-web, got %s", pm[9997])
	}
	if pm[6379] != "redis-cache" {
		t.Errorf("port 6379: expected redis-cache, got %s", pm[6379])
	}
	if pm[8080] != "app-server" {
		t.Errorf("port 8080: expected app-server, got %s", pm[8080])
	}
	if pm[8443] != "app-server" {
		t.Errorf("port 8443: expected app-server, got %s", pm[8443])
	}
}

func TestParseOutput_Empty(t *testing.T) {
	pm, _ := ParseOutput(strings.NewReader(""))
	if len(pm) != 0 {
		t.Fatalf("expected 0 mappings for empty input, got %d", len(pm))
	}
}

func TestParseHostPort(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"0.0.0.0:9997->9997/tcp", 9997},
		{":::9997->9997/tcp", 9997},
		{"[::]:8080->8080/tcp", 8080},
		{"127.0.0.1:3000->3000/tcp", 3000},
		{"noport", 0},
	}
	for _, tt := range tests {
		got := parseHostPort(tt.input)
		if got != tt.expected {
			t.Errorf("parseHostPort(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}
