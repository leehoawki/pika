package docker

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"
)

// PortMap maps host listen port → container name.
type PortMap map[int]string

// ParsePortMappings runs docker ps and builds host port → container name mapping.
func ParsePortMappings() (PortMap, []error) {
	cmd := exec.Command("docker", "ps", "--format", "{{.Names}}\t{{.Ports}}")
	output, err := cmd.Output()
	if err != nil {
		return nil, []error{fmt.Errorf("docker ps failed: %w", err)}
	}
	return ParseOutput(strings.NewReader(string(output)))
}

// ParseOutput parses docker ps format output into a PortMap.
// Input format per line: "container_name\t0.0.0.0:9997->9997/tcp, :::9997->9997/tcp"
func ParseOutput(r io.Reader) (PortMap, []error) {
	result := make(PortMap)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 2)
		if len(fields) < 2 {
			continue
		}
		containerName := fields[0]
		portsStr := fields[1]

		for _, mapping := range strings.Split(portsStr, ", ") {
			if !strings.Contains(mapping, "->") {
				continue
			}
			hostPort := parseHostPort(mapping)
			if hostPort > 0 {
				if _, exists := result[hostPort]; !exists {
					result[hostPort] = containerName
				}
			}
		}
	}

	return result, nil
}

// parseHostPort extracts the host-side port from a mapping like "0.0.0.0:9997->9997/tcp".
func parseHostPort(mapping string) int {
	left := strings.SplitN(mapping, "->", 2)[0]
	// left is "0.0.0.0:9997" or ":::9997" or "[::]:9997"
	colonIdx := strings.LastIndex(left, ":")
	if colonIdx < 0 {
		return 0
	}
	portStr := left[colonIdx+1:]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}
