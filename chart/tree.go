package chart

import (
	"fmt"
	"sort"
	"strings"

	"github.com/leehoawki/pika/model"
)

var stateOrder = []string{"TOTAL", "ESTABLISHED", "TIME_WAIT", "CLOSE_WAIT", "SYN_SENT", "SYN_RECV"}

func formatCounts(counts model.StateCounts) string {
	parts := []string{}
	added := map[string]bool{}

	for _, s := range stateOrder {
		if cnt, ok := counts[s]; ok {
			parts = append(parts, fmt.Sprintf("%s:%d", s, cnt))
			added[s] = true
		}
	}

	var extra []string
	for s := range counts {
		if !added[s] {
			extra = append(extra, s)
		}
	}
	sort.Strings(extra)
	for _, s := range extra {
		parts = append(parts, fmt.Sprintf("%s:%d", s, counts[s]))
	}

	return "(" + strings.Join(parts, " ") + ")"
}

func RenderTree(groups []model.ProcessGroup, dockerPorts map[int]string) string {
	if len(groups) == 0 {
		return "No connections found.\n"
	}

	var sb strings.Builder

	for gi, group := range groups {
		if gi > 0 {
			sb.WriteString("\n")
		}

		// Process header
		name := group.ProcessName
		if name == "" && group.PID == 0 {
			name = "(unknown)"
		}
		if group.PID > 0 {
			sb.WriteString(fmt.Sprintf("%s (PID: %d)\n", Colorize(Bold(name), Green), group.PID))
		} else {
			sb.WriteString(fmt.Sprintf("%s\n", Colorize(Bold(name), Green)))
		}

		// Collect all entries for this process (IN first, then OUT)
		type entry struct {
			isIn      bool
			port      int
			remotes   []model.RemoteStats
			outAddr   string
			outPort   int
			outCounts model.StateCounts
		}
		var entries []entry
		for _, l := range group.Listeners {
			entries = append(entries, entry{isIn: true, port: l.Port, remotes: l.Remotes})
		}
		for _, o := range group.Outbounds {
			entries = append(entries, entry{isIn: false, outAddr: o.RemoteAddr, outPort: o.RemotePort, outCounts: o.Counts})
		}

		for ei, e := range entries {
			connector := "├─"
			if ei == len(entries)-1 {
				connector = "└─"
			}

			if e.isIn {
				// IN entry: :PORT (IN) or container:PORT (IN) for docker-proxy
				portLabel := fmt.Sprintf(":%d", e.port)
				if group.ProcessName == "docker-proxy" && dockerPorts != nil {
					if containerName, ok := dockerPorts[e.port]; ok {
						portLabel = fmt.Sprintf("%s:%d", containerName, e.port)
					}
				}
				sb.WriteString(fmt.Sprintf("  %s %s %s\n",
					connector,
					Colorize(Bold(portLabel), Blue),
					Bold("(IN)"),
				))
				// Remote IPs under this port
				for ri, remote := range e.remotes {
					branch := "│   "
					if ei == len(entries)-1 {
						branch = "    "
					}
					rc := "├─"
					if ri == len(e.remotes)-1 {
						rc = "└─"
					}
					sb.WriteString(fmt.Sprintf("  %s %s %s %s\n",
						branch, rc,
						Colorize(remote.IP, Cyan),
						formatCounts(remote.Counts),
					))
				}
			} else {
				// OUT entry: IP:PORT (OUT) (counts)
				sb.WriteString(fmt.Sprintf("  %s %s %s %s\n",
					connector,
					Colorize(fmt.Sprintf("%s:%d", e.outAddr, e.outPort), Cyan),
					Bold("(OUT)"),
					formatCounts(e.outCounts),
				))
			}
		}
	}

	return sb.String()
}
