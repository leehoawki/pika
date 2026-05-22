package model

import (
	"testing"
)

func TestClassify_ProcessGroup(t *testing.T) {
	conns := []Connection{
		// nginx: LISTEN :80 + 2 inbound from 10.0.0.3 + 1 inbound from 10.0.0.5 + 1 outbound to 93.184.216.34:443
		{LocalPort: 80, ProcessName: "nginx", PID: 1234, State: "LISTEN"},
		{LocalPort: 80, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.3", RemotePort: 43210, State: "ESTABLISHED"},
		{LocalPort: 80, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.3", RemotePort: 43211, State: "TIME_WAIT"},
		{LocalPort: 80, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.5", RemotePort: 52301, State: "ESTABLISHED"},
		{LocalPort: 54000, ProcessName: "nginx", PID: 1234, RemoteAddr: "93.184.216.34", RemotePort: 443, State: "ESTABLISHED"},
		// sshd: LISTEN :22 + 1 inbound
		{LocalPort: 22, ProcessName: "sshd", PID: 910, State: "LISTEN"},
		{LocalPort: 22, ProcessName: "sshd", PID: 910, RemoteAddr: "192.168.1.100", RemotePort: 55123, State: "ESTABLISHED"},
		// anonymous: outbound only
		{LocalPort: 54322, ProcessName: "", PID: 0, RemoteAddr: "8.8.8.8", RemotePort: 53, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	// 3 groups: nginx (4 conns), sshd (1 conn), anonymous (1 conn)
	if len(groups) != 3 {
		t.Fatalf("expected 3 process groups, got %d", len(groups))
	}

	// Sorted by totalCount descending: nginx(4) > sshd(1) > anonymous(1)
	// sshd and anonymous both have 1, order between them is not strictly defined;
	// we just check nginx is first.
	if groups[0].ProcessName != "nginx" {
		t.Errorf("first group: expected nginx, got %s", groups[0].ProcessName)
	}
	if groups[0].PID != 1234 {
		t.Errorf("nginx PID: expected 1234, got %d", groups[0].PID)
	}

	// nginx: 1 listener (port 80), 1 outbound
	if len(groups[0].Listeners) != 1 {
		t.Fatalf("nginx listeners: expected 1, got %d", len(groups[0].Listeners))
	}
	if groups[0].Listeners[0].Port != 80 {
		t.Errorf("nginx listener port: expected 80, got %d", groups[0].Listeners[0].Port)
	}
	// 2 remote IPs on port 80
	if len(groups[0].Listeners[0].Remotes) != 2 {
		t.Fatalf("port 80 remotes: expected 2, got %d", len(groups[0].Listeners[0].Remotes))
	}
	// Remotes sorted by TOTAL descending: 10.0.0.3 (2) > 10.0.0.5 (1)
	if groups[0].Listeners[0].Remotes[0].IP != "10.0.0.3" {
		t.Errorf("first remote: expected 10.0.0.3, got %s", groups[0].Listeners[0].Remotes[0].IP)
	}
	if groups[0].Listeners[0].Remotes[0].Counts["TOTAL"] != 2 {
		t.Errorf("10.0.0.3 total: expected 2, got %d", groups[0].Listeners[0].Remotes[0].Counts["TOTAL"])
	}

	if len(groups[0].Outbounds) != 1 {
		t.Fatalf("nginx outbounds: expected 1, got %d", len(groups[0].Outbounds))
	}
	if groups[0].Outbounds[0].RemoteAddr != "93.184.216.34" || groups[0].Outbounds[0].RemotePort != 443 {
		t.Errorf("nginx outbound: expected 93.184.216.34:443, got %s:%d",
			groups[0].Outbounds[0].RemoteAddr, groups[0].Outbounds[0].RemotePort)
	}

	// Find sshd group (either index 1 or 2)
	var sshdGroup *ProcessGroup
	var anonGroup *ProcessGroup
	for i := range groups {
		if groups[i].ProcessName == "sshd" {
			sshdGroup = &groups[i]
		}
		if groups[i].ProcessName == "" && groups[i].PID == 0 {
			anonGroup = &groups[i]
		}
	}

	if sshdGroup == nil {
		t.Fatal("sshd group not found")
	}
	if len(sshdGroup.Listeners) != 1 {
		t.Fatalf("sshd listeners: expected 1, got %d", len(sshdGroup.Listeners))
	}
	if sshdGroup.Listeners[0].Port != 22 {
		t.Errorf("sshd listener port: expected 22, got %d", sshdGroup.Listeners[0].Port)
	}
	if len(sshdGroup.Listeners[0].Remotes) != 1 {
		t.Fatalf("port 22 remotes: expected 1, got %d", len(sshdGroup.Listeners[0].Remotes))
	}
	if sshdGroup.Listeners[0].Remotes[0].IP != "192.168.1.100" {
		t.Errorf("port 22 remote: expected 192.168.1.100, got %s", sshdGroup.Listeners[0].Remotes[0].IP)
	}
	if len(sshdGroup.Outbounds) != 0 {
		t.Errorf("sshd outbounds: expected 0, got %d", len(sshdGroup.Outbounds))
	}

	if anonGroup == nil {
		t.Fatal("anonymous group not found")
	}
	if len(anonGroup.Listeners) != 0 {
		t.Errorf("anonymous listeners: expected 0, got %d", len(anonGroup.Listeners))
	}
	if len(anonGroup.Outbounds) != 1 {
		t.Fatalf("anonymous outbounds: expected 1, got %d", len(anonGroup.Outbounds))
	}
	if anonGroup.Outbounds[0].RemoteAddr != "8.8.8.8" || anonGroup.Outbounds[0].RemotePort != 53 {
		t.Errorf("anonymous outbound: expected 8.8.8.8:53, got %s:%d",
			anonGroup.Outbounds[0].RemoteAddr, anonGroup.Outbounds[0].RemotePort)
	}
}

func TestClassify_ListenOnlyHidden(t *testing.T) {
	conns := []Connection{
		{LocalPort: 3306, ProcessName: "mysqld", PID: 5678, State: "LISTEN"},
	}

	groups := Classify(conns)

	// LISTEN without connections should produce 0 groups
	if len(groups) != 0 {
		t.Errorf("expected 0 groups (LISTEN only), got %d", len(groups))
	}
}

func TestClassify_Empty(t *testing.T) {
	groups := Classify(nil)
	if len(groups) != 0 {
		t.Errorf("expected 0 groups for nil input, got %d", len(groups))
	}
}

func TestClassify_UDPEmptyRemote(t *testing.T) {
	conns := []Connection{
		// UDP with empty remotes should be excluded
		{Protocol: "udp", LocalPort: 68, ProcessName: "dhclient", PID: 989, RemoteAddr: "0.0.0.0", RemotePort: 0, State: ""},
		// UDPv6 with empty remotes should be excluded
		{Protocol: "udp6", LocalPort: 5353, ProcessName: "avahi", PID: 550, RemoteAddr: "::", RemotePort: 0, State: ""},
		// A normal outbound connection
		{Protocol: "tcp", LocalPort: 54322, RemoteAddr: "8.8.8.8", RemotePort: 53, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Listeners) != 0 {
		t.Errorf("expected 0 listeners (UDP empty remotes excluded), got %d", len(groups[0].Listeners))
	}
	if len(groups[0].Outbounds) != 1 {
		t.Fatalf("expected 1 outbound, got %d", len(groups[0].Outbounds))
	}
	if groups[0].Outbounds[0].RemoteAddr != "8.8.8.8" {
		t.Errorf("expected 8.8.8.8, got %s", groups[0].Outbounds[0].RemoteAddr)
	}
}

func TestClassify_SortByTotalCount(t *testing.T) {
	conns := []Connection{
		// proc_a: 1 connection
		{LocalPort: 50001, ProcessName: "proc_a", PID: 100, RemoteAddr: "1.2.3.4", RemotePort: 80, State: "ESTABLISHED"},
		// proc_b: 3 connections
		{LocalPort: 50002, ProcessName: "proc_b", PID: 200, RemoteAddr: "5.6.7.8", RemotePort: 443, State: "ESTABLISHED"},
		{LocalPort: 50003, ProcessName: "proc_b", PID: 200, RemoteAddr: "5.6.7.8", RemotePort: 443, State: "TIME_WAIT"},
		{LocalPort: 50004, ProcessName: "proc_b", PID: 200, RemoteAddr: "9.10.11.12", RemotePort: 8080, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// proc_b (3 conns) should come before proc_a (1 conn)
	if groups[0].ProcessName != "proc_b" {
		t.Errorf("first group: expected proc_b, got %s", groups[0].ProcessName)
	}
	if totalCount(groups[0]) != 3 {
		t.Errorf("proc_b total: expected 3, got %d", totalCount(groups[0]))
	}
	if groups[1].ProcessName != "proc_a" {
		t.Errorf("second group: expected proc_a, got %s", groups[1].ProcessName)
	}
}

func TestClassify_SameProcessDifferentPID(t *testing.T) {
	conns := []Connection{
		{LocalPort: 50001, ProcessName: "worker", PID: 100, RemoteAddr: "1.2.3.4", RemotePort: 80, State: "ESTABLISHED"},
		{LocalPort: 50002, ProcessName: "worker", PID: 200, RemoteAddr: "5.6.7.8", RemotePort: 443, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	if len(groups) != 2 {
		t.Fatalf("expected 2 groups (same name, different PID), got %d", len(groups))
	}

	pids := map[int]bool{}
	for _, g := range groups {
		if g.ProcessName != "worker" {
			t.Errorf("expected worker, got %s", g.ProcessName)
		}
		pids[g.PID] = true
	}
	if !pids[100] || !pids[200] {
		t.Errorf("expected PIDs 100 and 200, got %v", pids)
	}
}

func TestClassify_NoProcessInfo(t *testing.T) {
	conns := []Connection{
		{LocalPort: 50001, ProcessName: "", PID: 0, RemoteAddr: "1.2.3.4", RemotePort: 80, State: "ESTABLISHED"},
		{LocalPort: 50002, ProcessName: "", PID: 0, RemoteAddr: "5.6.7.8", RemotePort: 443, State: "TIME_WAIT"},
	}

	groups := Classify(conns)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group (empty name + PID 0), got %d", len(groups))
	}
	if len(groups[0].Outbounds) != 2 {
		t.Fatalf("expected 2 outbounds, got %d", len(groups[0].Outbounds))
	}
	// Sorted by TOTAL descending; both have 1, so order is not strictly defined
	total := 0
	for _, o := range groups[0].Outbounds {
		total += o.Counts["TOTAL"]
	}
	if total != 2 {
		t.Errorf("expected total count 2, got %d", total)
	}
}

func TestClassify_InferUnknownToKnownProcess(t *testing.T) {
	conns := []Connection{
		// nginx has ESTABLISHED outbound to 10.0.0.100:3306
		{LocalPort: 54000, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "ESTABLISHED"},
		// unknown (PID=0) has TIME_WAIT to the same target 10.0.0.100:3306
		{LocalPort: 54001, ProcessName: "", PID: 0, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "TIME_WAIT"},
		{LocalPort: 54002, ProcessName: "", PID: 0, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "TIME_WAIT"},
		// unknown to a different target that nginx also doesn't have → stays unknown
	}

	groups := Classify(conns)

	// Should produce only 1 group: nginx (unknown merged in)
	if len(groups) != 1 {
		t.Fatalf("expected 1 group (unknown inferred to nginx), got %d", len(groups))
	}
	if groups[0].ProcessName != "nginx" {
		t.Errorf("expected nginx, got %s", groups[0].ProcessName)
	}
	if len(groups[0].Outbounds) != 1 {
		t.Fatalf("expected 1 outbound entry, got %d", len(groups[0].Outbounds))
	}
	ob := groups[0].Outbounds[0]
	if ob.RemoteAddr != "10.0.0.100" || ob.RemotePort != 3306 {
		t.Errorf("expected 10.0.0.100:3306, got %s:%d", ob.RemoteAddr, ob.RemotePort)
	}
	// TOTAL should be 3 (1 ESTABLISHED + 2 TIME_WAIT)
	if ob.Counts["TOTAL"] != 3 {
		t.Errorf("expected TOTAL 3, got %d", ob.Counts["TOTAL"])
	}
	if ob.Counts["ESTABLISHED"] != 1 {
		t.Errorf("expected ESTABLISHED 1, got %d", ob.Counts["ESTABLISHED"])
	}
	if ob.Counts["TIME_WAIT"] != 2 {
		t.Errorf("expected TIME_WAIT 2, got %d", ob.Counts["TIME_WAIT"])
	}
}

func TestClassify_InferAmbiguousTarget(t *testing.T) {
	conns := []Connection{
		// nginx connects to 10.0.0.100:3306
		{LocalPort: 54000, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "ESTABLISHED"},
		// php-fpm also connects to 10.0.0.100:3306
		{LocalPort: 54001, ProcessName: "php-fpm", PID: 5678, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "ESTABLISHED"},
		// unknown TIME_WAIT to the same target → ambiguous, should NOT infer
		{LocalPort: 54002, ProcessName: "", PID: 0, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "TIME_WAIT"},
	}

	groups := Classify(conns)

	// Should have 3 groups: nginx, php-fpm, unknown
	if len(groups) != 3 {
		t.Fatalf("expected 3 groups (ambiguous target, no inference), got %d", len(groups))
	}

	// Verify unknown group still exists with 1 outbound
	var unknownGroup *ProcessGroup
	for i := range groups {
		if groups[i].ProcessName == "" && groups[i].PID == 0 {
			unknownGroup = &groups[i]
		}
	}
	if unknownGroup == nil {
		t.Fatal("expected unknown group to remain (ambiguous target)")
	}
	if len(unknownGroup.Outbounds) != 1 {
		t.Fatalf("expected 1 unknown outbound, got %d", len(unknownGroup.Outbounds))
	}
}

func TestClassify_InferNoMatch(t *testing.T) {
	conns := []Connection{
		// nginx connects to 10.0.0.100:3306
		{LocalPort: 54000, ProcessName: "nginx", PID: 1234, RemoteAddr: "10.0.0.100", RemotePort: 3306, State: "ESTABLISHED"},
		// unknown TIME_WAIT to a completely different target → no match, stays unknown
		{LocalPort: 54001, ProcessName: "", PID: 0, RemoteAddr: "10.0.0.200", RemotePort: 6379, State: "TIME_WAIT"},
	}

	groups := Classify(conns)

	// Should have 2 groups: nginx and unknown
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	var unknownGroup *ProcessGroup
	for i := range groups {
		if groups[i].ProcessName == "" && groups[i].PID == 0 {
			unknownGroup = &groups[i]
		}
	}
	if unknownGroup == nil {
		t.Fatal("expected unknown group to remain (no matching target)")
	}
	if len(unknownGroup.Outbounds) != 1 {
		t.Fatalf("expected 1 unknown outbound, got %d", len(unknownGroup.Outbounds))
	}
	if unknownGroup.Outbounds[0].RemoteAddr != "10.0.0.200" {
		t.Errorf("expected 10.0.0.200, got %s", unknownGroup.Outbounds[0].RemoteAddr)
	}
}

func TestClassify_Wildcard0000(t *testing.T) {
	// Simulate: LISTEN on 0.0.0.0:22 by sshd, ESTABLISHED on 10.0.0.1:22 (different local addr)
	conns := []Connection{
		{Protocol: "tcp", LocalAddr: "0.0.0.0", LocalPort: 22, ProcessName: "sshd", PID: 100, State: "LISTEN"},
		{Protocol: "tcp", LocalAddr: "10.0.0.1", LocalPort: 22, ProcessName: "sshd", PID: 100, RemoteAddr: "192.168.1.5", RemotePort: 55123, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Listeners) != 1 {
		t.Fatalf("expected 1 listener (IN), got %d", len(groups[0].Listeners))
	}
	if groups[0].Listeners[0].Port != 22 {
		t.Errorf("listener port: expected 22, got %d", groups[0].Listeners[0].Port)
	}
}

func TestClassify_PIDPortMatch(t *testing.T) {
	// PID-based fallback: same PID owns LISTEN on tcp6 :::8080, connection is tcp on 10.0.0.1:8080
	conns := []Connection{
		{Protocol: "tcp6", LocalAddr: "::", LocalPort: 8080, ProcessName: "app", PID: 500, State: "LISTEN"},
		{Protocol: "tcp", LocalAddr: "10.0.0.1", LocalPort: 8080, ProcessName: "app", PID: 500, RemoteAddr: "1.2.3.4", RemotePort: 9988, State: "ESTABLISHED"},
	}

	groups := Classify(conns)

	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Listeners) != 1 {
		t.Fatalf("expected 1 listener (IN via PID match), got %d", len(groups[0].Listeners))
	}
	if groups[0].Listeners[0].Port != 8080 {
		t.Errorf("listener port: expected 8080, got %d", groups[0].Listeners[0].Port)
	}
}
