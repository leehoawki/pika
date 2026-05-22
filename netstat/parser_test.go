package netstat

import (
	"strings"
	"testing"
)

func TestParseOutput_UDP_Skipped(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
udp        0      0 0.0.0.0:53              0.0.0.0:*                           999/dnsmasq
tcp        0      0 0.0.0.0:80              0.0.0.0:*               LISTEN      1234/nginx
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(conns) != 1 {
		t.Fatalf("expected 1 TCP connection (UDP skipped), got %d", len(conns))
	}
	if conns[0].Protocol != "tcp" {
		t.Errorf("expected tcp, got %q", conns[0].Protocol)
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (UDP skipped), got %d", len(errs))
	}
}

func TestParseOutput_TCP(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:80              0.0.0.0:*               LISTEN      1234/nginx
tcp        0      0 192.168.1.10:80         10.0.0.5:43210          ESTABLISHED 1234/nginx
tcp        0      0 192.168.1.10:22         192.168.1.5:55123       ESTABLISHED 910/sshd
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(conns) != 3 {
		t.Fatalf("expected 3 connections, got %d", len(conns))
	}

	// 验证第一条 LISTEN
	c := conns[0]
	if c.Protocol != "tcp" {
		t.Errorf("proto: got %q, want %q", c.Protocol, "tcp")
	}
	if c.LocalAddr != "0.0.0.0" {
		t.Errorf("local addr: got %q, want %q", c.LocalAddr, "0.0.0.0")
	}
	if c.LocalPort != 80 {
		t.Errorf("local port: got %d, want %d", c.LocalPort, 80)
	}
	if c.State != "LISTEN" {
		t.Errorf("state: got %q, want %q", c.State, "LISTEN")
	}
	if c.PID != 1234 {
		t.Errorf("pid: got %d, want %d", c.PID, 1234)
	}
	if c.ProcessName != "nginx" {
		t.Errorf("process: got %q, want %q", c.ProcessName, "nginx")
	}

	// 验证 ESTABLISHED 连接的远程地址
	c = conns[1]
	if c.RemoteAddr != "10.0.0.5" {
		t.Errorf("remote addr: got %q, want %q", c.RemoteAddr, "10.0.0.5")
	}
	if c.RemotePort != 43210 {
		t.Errorf("remote port: got %d, want %d", c.RemotePort, 43210)
	}
}

func TestParseOutput_NoPermission(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:80              0.0.0.0:*               LISTEN      -
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	c := conns[0]
	if c.PID != 0 {
		t.Errorf("PID should be 0 for '-', got %d", c.PID)
	}
	if c.ProcessName != "" {
		t.Errorf("ProcessName should be empty for '-', got %q", c.ProcessName)
	}
}

func TestParseOutput_IPv6(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp6       0      0 :::8080                 :::*                    LISTEN      5678/app
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(conns) != 1 {
		t.Fatalf("expected 1 connection, got %d", len(conns))
	}
	c := conns[0]
	if c.Protocol != "tcp6" {
		t.Errorf("proto: got %q, want %q", c.Protocol, "tcp6")
	}
	if c.LocalPort != 8080 {
		t.Errorf("local port: got %d, want %d", c.LocalPort, 8080)
	}
}

func TestParseOutput_SkipsUnparseable(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp        0      0 0.0.0.0:80              0.0.0.0:*               LISTEN      1234/nginx
GARBAGE LINE HERE
tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      910/sshd
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}
	if len(errs) != 1 {
		t.Fatalf("expected 1 parse error, got %d", len(errs))
	}
}

func TestParseOutput_Empty(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(conns) != 0 {
		t.Fatalf("expected 0 connections, got %d", len(conns))
	}
}

func TestParseAddress(t *testing.T) {
	tests := []struct {
		input    string
		wantIP   string
		wantPort int
	}{
		{"0.0.0.0:80", "0.0.0.0", 80},
		{"192.168.1.10:443", "192.168.1.10", 443},
		{":::8080", "::", 8080},
		{"0.0.0.0:*", "0.0.0.0", 0},
		{"10.0.0.5:43210", "10.0.0.5", 43210},
	}

	for _, tt := range tests {
		ip, port := parseAddress(tt.input)
		if ip != tt.wantIP {
			t.Errorf("parseAddress(%q) ip = %q, want %q", tt.input, ip, tt.wantIP)
		}
		if port != tt.wantPort {
			t.Errorf("parseAddress(%q) port = %d, want %d", tt.input, port, tt.wantPort)
		}
	}
}

func TestParseOutput_TCP6_Established(t *testing.T) {
	input := `Active Internet connections (servers and established)
Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
tcp6       0      0 :::13306                :::*                    LISTEN      2145/mysqld
tcp6       0      0 ::1:13306               ::1:54321               ESTABLISHED 2145/mysqld
`
	conns, errs := ParseOutput(strings.NewReader(input))
	if len(errs) > 0 {
		t.Fatalf("unexpected parse errors: %v", errs)
	}
	if len(conns) != 2 {
		t.Fatalf("expected 2 connections, got %d", len(conns))
	}

	// Verify LISTEN entry
	if conns[0].Protocol != "tcp6" {
		t.Errorf("LISTEN proto: got %q, want tcp6", conns[0].Protocol)
	}
	if conns[0].LocalPort != 13306 {
		t.Errorf("LISTEN port: got %d, want 13306", conns[0].LocalPort)
	}
	if conns[0].State != "LISTEN" {
		t.Errorf("LISTEN state: got %q, want LISTEN", conns[0].State)
	}
	if conns[0].PID != 2145 {
		t.Errorf("LISTEN PID: got %d, want 2145", conns[0].PID)
	}
	if conns[0].ProcessName != "mysqld" {
		t.Errorf("LISTEN process: got %q, want mysqld", conns[0].ProcessName)
	}

	// Verify ESTABLISHED entry
	if conns[1].Protocol != "tcp6" {
		t.Errorf("ESTABLISHED proto: got %q, want tcp6", conns[1].Protocol)
	}
	if conns[1].LocalAddr != "::1" {
		t.Errorf("ESTABLISHED local addr: got %q, want ::1", conns[1].LocalAddr)
	}
	if conns[1].LocalPort != 13306 {
		t.Errorf("ESTABLISHED local port: got %d, want 13306", conns[1].LocalPort)
	}
	if conns[1].RemoteAddr != "::1" {
		t.Errorf("ESTABLISHED remote addr: got %q, want ::1", conns[1].RemoteAddr)
	}
	if conns[1].RemotePort != 54321 {
		t.Errorf("ESTABLISHED remote port: got %d, want 54321", conns[1].RemotePort)
	}
	if conns[1].State != "ESTABLISHED" {
		t.Errorf("ESTABLISHED state: got %q, want ESTABLISHED", conns[1].State)
	}
	if conns[1].PID != 2145 {
		t.Errorf("ESTABLISHED PID: got %d, want 2145", conns[1].PID)
	}
	if conns[1].ProcessName != "mysqld" {
		t.Errorf("ESTABLISHED process: got %q, want mysqld", conns[1].ProcessName)
	}
}
