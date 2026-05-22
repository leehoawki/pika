package netstat

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"strings"

	"github.com/leehoawki/pika/model"
)

// Parse 执行 netstat -tnap 并解析输出为连接列表（仅 TCP）。
func Parse() ([]model.Connection, []error) {
	cmd := exec.Command("netstat", "-tnap")
	output, err := cmd.Output()
	if err != nil {
		return nil, []error{fmt.Errorf("netstat not found. Install net-tools: sudo apt install net-tools")}
	}
	return ParseOutput(strings.NewReader(string(output)))
}

// ParseOutput 从 reader 解析 netstat 输出。
func ParseOutput(r io.Reader) ([]model.Connection, []error) {
	var conns []model.Connection
	var errs []error
	scanner := bufio.NewScanner(r)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过前两行表头
		if lineNum <= 2 {
			continue
		}
		if line == "" {
			continue
		}

		conn, err := parseLine(line)
		if err != nil {
			errs = append(errs, fmt.Errorf("line %d: %w", lineNum, err))
			continue
		}
		conns = append(conns, conn)
	}

	return conns, errs
}

func parseLine(line string) (model.Connection, error) {
	fields := strings.Fields(line)
	// 至少需要: Proto Recv-Q Send-Q Local Foreign [State] PID
	if len(fields) < 6 {
		return model.Connection{}, fmt.Errorf("too few fields: %q", line)
	}

	proto := fields[0]
	if !strings.HasPrefix(proto, "tcp") {
		return model.Connection{}, fmt.Errorf("skipping non-TCP: %q", line)
	}
	localAddr, localPort := parseAddress(fields[3])
	remoteAddr, remotePort := parseAddress(fields[4])

	if len(fields) < 7 {
		return model.Connection{}, fmt.Errorf("too few fields for TCP: %q", line)
	}
	state := fields[5]
	pidStr := fields[6]

	pid := 0
	processName := ""
	if pidStr != "-" && pidStr != "" {
		parts := strings.SplitN(pidStr, "/", 2)
		pid, _ = strconv.Atoi(parts[0])
		if len(parts) == 2 {
			processName = parts[1]
		}
	}

	return model.Connection{
		Protocol:    proto,
		LocalAddr:   localAddr,
		LocalPort:   localPort,
		RemoteAddr:  remoteAddr,
		RemotePort:  remotePort,
		State:       state,
		PID:         pid,
		ProcessName: processName,
	}, nil
}

// parseAddress 将 "IP:port" 格式拆分为 IP 和端口。"*" 端口返回 0。
func parseAddress(addr string) (string, int) {
	idx := strings.LastIndex(addr, ":")
	if idx == -1 {
		return addr, 0
	}
	ip := addr[:idx]
	portStr := addr[idx+1:]
	if portStr == "*" {
		return ip, 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return addr, 0
	}
	return ip, port
}
