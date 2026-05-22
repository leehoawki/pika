package main

import (
	"fmt"
	"os"

	"github.com/leehoawki/pika/chart"
	"github.com/leehoawki/pika/docker"
	"github.com/leehoawki/pika/filter"
	"github.com/leehoawki/pika/model"
	"github.com/leehoawki/pika/netstat"
	"github.com/leehoawki/pika/process"
)

var version = "0.1.0"

func main() {
	args := os.Args[1:]

	var (
		procName string
		showHelp bool
		showVer  bool
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--process":
			i++
			if i < len(args) {
				procName = args[i]
			}
		case "-h", "--help":
			showHelp = true
		case "-v", "--version":
			showVer = true
		default:
			fmt.Fprintf(os.Stderr, "Unknown option: %s\n", args[i])
			printHelp()
			os.Exit(1)
		}
	}

	if showHelp {
		printHelp()
		return
	}
	if showVer {
		fmt.Printf("pika version %s\n", version)
		return
	}

	conns, errs := netstat.Parse()
	for _, e := range errs {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", e)
	}
	if len(errs) > 0 && len(conns) == 0 {
		fmt.Fprintf(os.Stderr, "Error: %v\n", errs[0])
		os.Exit(1)
	}

	// ps -ef is optional — parent resolution is best-effort
	if pTree, pErrs := process.Parse(); len(pErrs) == 0 && pTree != nil {
		resolveParentConnections(conns, pTree)
	}

	filtered := filter.Apply(conns, filter.Opts{
		Process: procName,
	})

	if len(filtered) == 0 {
		if procName != "" {
			fmt.Println("No connections match the filter criteria.")
		} else {
			fmt.Println("No connections found.")
		}
		return
	}

	hasPID := false
	for _, c := range filtered {
		if c.PID > 0 {
			hasPID = true
			break
		}
	}
	if !hasPID {
		fmt.Fprintln(os.Stderr, "Warning: run with sudo to see process info")
	}

	groups := model.Classify(filtered)

	// docker port mapping is optional
	var dockerPorts map[int]string
	if dp, dErrs := docker.ParsePortMappings(); len(dErrs) == 0 {
		dockerPorts = dp
	}

	fmt.Print(chart.RenderTree(groups, dockerPorts))
}

func printHelp() {
	help := `pika - Network Connection Visualizer

Usage:
  pika [options]

Options:
  --process NAME     Filter by process name (substring match)
  -h, --help         Show help
  -v, --version      Show version

Examples:
  sudo pika                        # Show connection tree
  sudo pika --process nginx        # Filter by process name
`
	fmt.Print(help)
}

// resolveParentConnections uses ps -ef data to merge child processes into their parent.
// Rules:
//   - LISTEN connections: only clean the name from ps -ef, never change PID
//   - Non-LISTEN connections: only merge to parent when local port matches parent's LISTEN port
//
// This avoids over-aggregation (e.g. nginx→systemd) while correctly merging
// sshd-session→sshd and nginx-worker→nginx on matching ports.
func resolveParentConnections(conns []model.Connection, pTree process.Tree) {
	// Build PID → set of LISTEN ports
	listenPorts := make(map[int]map[int]bool)
	for _, c := range conns {
		if c.State == "LISTEN" && c.PID > 0 {
			if listenPorts[c.PID] == nil {
				listenPorts[c.PID] = make(map[int]bool)
			}
			listenPorts[c.PID][c.LocalPort] = true
		}
	}

	for i := range conns {
		c := &conns[i]
		if c.PID <= 0 {
			continue
		}
		pInfo, ok := pTree[c.PID]
		if !ok {
			continue
		}

		// Always clean process name from ps -ef so same PID has consistent name
		c.ProcessName = pInfo.Name

		if c.State == "LISTEN" {
			continue
		}

		// Non-LISTEN: merge only if local port matches parent's LISTEN port
		parentPorts, parentIsListen := listenPorts[pInfo.PPID]
		if !parentIsListen || !parentPorts[c.LocalPort] {
			continue
		}
		parentInfo, ok := pTree[pInfo.PPID]
		if !ok {
			continue
		}
		c.ProcessName = parentInfo.Name
		c.PID = pInfo.PPID
	}
}
