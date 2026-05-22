package chart

import (
	"regexp"
	"strings"
	"testing"

	"github.com/leehoawki/pika/model"
)

// stripANSI removes ANSI escape sequences from a string.
var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func TestRenderTree_ProcessGroup(t *testing.T) {
	groups := []model.ProcessGroup{
		{
			ProcessName: "nginx",
			PID:         1234,
			Listeners: []model.ListenerEntry{
				{
					Port: 80,
					Remotes: []model.RemoteStats{
						{IP: "10.0.0.3", Counts: model.StateCounts{"TOTAL": 2, "ESTABLISHED": 1, "TIME_WAIT": 1}},
						{IP: "10.0.0.5", Counts: model.StateCounts{"TOTAL": 1, "ESTABLISHED": 1}},
					},
				},
			},
			Outbounds: []model.OutboundEntry{
				{RemoteAddr: "8.8.8.8", RemotePort: 53, Counts: model.StateCounts{"TOTAL": 2, "ESTABLISHED": 1, "TIME_WAIT": 1}},
			},
		},
		{
			ProcessName: "sshd",
			PID:         910,
			Listeners: []model.ListenerEntry{
				{
					Port: 22,
					Remotes: []model.RemoteStats{
						{IP: "192.168.1.100", Counts: model.StateCounts{"TOTAL": 1, "ESTABLISHED": 1}},
					},
				},
			},
		},
	}

	output := stripANSI(RenderTree(groups, nil))

	// Process headers
	if !strings.Contains(output, "nginx (PID: 1234)") {
		t.Error("expected nginx process header")
	}
	if !strings.Contains(output, "sshd (PID: 910)") {
		t.Error("expected sshd process header")
	}

	// IN entries
	if !strings.Contains(output, ":80 (IN)") {
		t.Error("expected :80 (IN)")
	}
	if !strings.Contains(output, ":22 (IN)") {
		t.Error("expected :22 (IN)")
	}

	// Remote IPs
	if !strings.Contains(output, "10.0.0.3") {
		t.Error("expected remote IP 10.0.0.3")
	}
	if !strings.Contains(output, "192.168.1.100") {
		t.Error("expected remote IP 192.168.1.100")
	}

	// OUT entries
	if !strings.Contains(output, "8.8.8.8:53 (OUT)") {
		t.Error("expected 8.8.8.8:53 (OUT)")
	}

	// Tree connectors
	if !strings.Contains(output, "├─") || !strings.Contains(output, "└─") {
		t.Error("expected tree connectors")
	}

	// State counts
	if !strings.Contains(output, "TOTAL:2") {
		t.Error("expected TOTAL:2")
	}
}

func TestRenderTree_Empty(t *testing.T) {
	output := RenderTree(nil, nil)
	if !strings.Contains(output, "No connections found") {
		t.Error("expected empty message")
	}
}

func TestRenderTree_UnknownProcess(t *testing.T) {
	groups := []model.ProcessGroup{
		{
			ProcessName: "",
			PID:         0,
			Outbounds: []model.OutboundEntry{
				{RemoteAddr: "1.1.1.1", RemotePort: 443, Counts: model.StateCounts{"TOTAL": 1, "ESTABLISHED": 1}},
			},
		},
	}

	output := stripANSI(RenderTree(groups, nil))

	if !strings.Contains(output, "(unknown)") {
		t.Error("expected (unknown) for no-process-info group")
	}
	if !strings.Contains(output, "1.1.1.1:443 (OUT)") {
		t.Error("expected outbound target")
	}
}

func TestRenderTree_OutboundOnly(t *testing.T) {
	groups := []model.ProcessGroup{
		{
			ProcessName: "curl",
			PID:         500,
			Outbounds: []model.OutboundEntry{
				{RemoteAddr: "9.9.9.9", RemotePort: 80, Counts: model.StateCounts{"TOTAL": 1, "ESTABLISHED": 1}},
			},
		},
	}

	output := stripANSI(RenderTree(groups, nil))

	if strings.Contains(output, "(IN)") {
		t.Error("expected no IN entries")
	}
	if !strings.Contains(output, "9.9.9.9:80 (OUT)") {
		t.Error("expected outbound target")
	}
}

func TestRenderTree_DockerProxy(t *testing.T) {
	groups := []model.ProcessGroup{
		{
			ProcessName: "docker-proxy",
			PID:         148593,
			Listeners: []model.ListenerEntry{
				{
					Port: 9997,
					Remotes: []model.RemoteStats{
						{IP: "10.0.0.3", Counts: model.StateCounts{"TOTAL": 5, "ESTABLISHED": 5}},
					},
				},
			},
		},
	}

	dockerPorts := map[int]string{9997: "nginx-web"}

	output := stripANSI(RenderTree(groups, dockerPorts))
	if !strings.Contains(output, "nginx-web:9997 (IN)") {
		t.Errorf("expected 'nginx-web:9997 (IN)', got:\n%s", output)
	}

	outputNoDocker := stripANSI(RenderTree(groups, nil))
	if !strings.Contains(outputNoDocker, ":9997 (IN)") {
		t.Errorf("expected ':9997 (IN)' without docker, got:\n%s", outputNoDocker)
	}
}
