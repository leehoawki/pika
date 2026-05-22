package model

import "sort"

// Connection 表示一条网络连接。
type Connection struct {
	Protocol    string
	LocalAddr   string
	LocalPort   int
	RemoteAddr  string
	RemotePort  int
	State       string
	PID         int
	ProcessName string
}

// StateCounts 记录各连接状态的数量。
type StateCounts map[string]int

// ProcessGroup 按进程聚合的所有连接。
type ProcessGroup struct {
	ProcessName string
	PID         int
	Listeners   []ListenerEntry
	Outbounds   []OutboundEntry
}

// ListenerEntry 表示一个监听端口上的入站连接。
type ListenerEntry struct {
	Port    int
	Remotes []RemoteStats
}

// OutboundEntry 表示一个出站连接目标。
type OutboundEntry struct {
	RemoteAddr string
	RemotePort int
	Counts     StateCounts
}

// RemoteStats 按远程 IP 聚合的状态计数。
type RemoteStats struct {
	IP     string
	Counts StateCounts
}

// hasRemote 判断连接是否有真实的远程端点（排除 LISTEN 状态的空地址）。
// 注意：:: 是 IPv6 的有效地址（如 ::1 表示 localhost），只有在 LISTEN 连接中 :: 才表示"监听所有地址"
func hasRemote(c Connection) bool {
	return c.RemotePort != 0 && c.RemoteAddr != "" && c.RemoteAddr != "*" && c.RemoteAddr != "0.0.0.0"
}

// totalCount 计算 ProcessGroup 中所有连接的总数。
func totalCount(pg ProcessGroup) int {
	total := 0
	for _, l := range pg.Listeners {
		for _, r := range l.Remotes {
			if cnt, ok := r.Counts["TOTAL"]; ok {
				total += cnt
			}
		}
	}
	for _, o := range pg.Outbounds {
		if cnt, ok := o.Counts["TOTAL"]; ok {
			total += cnt
		}
	}
	return total
}

// Classify 将连接按进程分组，再分为入站（Listener）和出站（Outbound）两类。
// 入站：本地端口处于 LISTEN 状态且该端口有外部连接。
// 出站：本地端口没有对应的 LISTEN，属于本机主动发起的连接。
func Classify(conns []Connection) []ProcessGroup {
	// 进程键（提前定义，供后续使用）
	type processKey struct {
		name string
		pid  int
	}

	// 1. 收集所有 LISTEN 端口（按协议和本地地址区分）
	// key: "protocol:localAddr:port"，value: processKey
	type listenKey struct {
		proto     string
		localAddr string
		port      int
	}
	listenOwners := make(map[listenKey]processKey)
	for _, c := range conns {
		if c.State == "LISTEN" {
			lk := listenKey{c.Protocol, c.LocalAddr, c.LocalPort}
			listenOwners[lk] = processKey{c.ProcessName, c.PID}
		}
	}

	// 入站：按 (processKey, port, remoteIP) 聚合
	type inboundKey struct {
		pk    processKey
		port  int
		remIP string
	}
	inboundCounts := make(map[inboundKey]StateCounts)

	// 出站：按 (processKey, remoteAddr, remotePort) 聚合
	type outboundKey struct {
		pk      processKey
		remAddr string
		remPort int
	}
	outboundCounts := make(map[outboundKey]StateCounts)

	for _, c := range conns {
		if c.State == "LISTEN" || !hasRemote(c) {
			continue
		}
		pk := processKey{c.ProcessName, c.PID}
		// 检查是否有 LISTEN 监听此端口
		// 优先匹配精确的 (protocol, localAddr, port)，然后尝试通配符
		var owner processKey
		var found bool
		lk := listenKey{c.Protocol, c.LocalAddr, c.LocalPort}
		if owner, found = listenOwners[lk]; !found {
			// 尝试匹配通配符地址（IPv6 ::）
			lkWildcard := listenKey{c.Protocol, "::", c.LocalPort}
			owner, found = listenOwners[lkWildcard]
		}
		if !found {
			// 尝试匹配通配符地址（IPv4 0.0.0.0）
			lkWildcard4 := listenKey{c.Protocol, "0.0.0.0", c.LocalPort}
			owner, found = listenOwners[lkWildcard4]
		}
		if !found {
			// PID+端口匹配：同一 PID 在同一端口有 LISTEN
			for l, pk := range listenOwners {
				if pk.pid == c.PID && l.port == c.LocalPort {
					owner = pk
					found = true
					break
				}
			}
		}
		if found {
			// 入站连接：归入监听端口的进程
			key := inboundKey{owner, c.LocalPort, c.RemoteAddr}
			if inboundCounts[key] == nil {
				inboundCounts[key] = make(StateCounts)
			}
			inboundCounts[key][c.State]++
		} else {
			// 出站连接
			key := outboundKey{pk, c.RemoteAddr, c.RemotePort}
			if outboundCounts[key] == nil {
				outboundCounts[key] = make(StateCounts)
			}
			outboundCounts[key][c.State]++
		}
	}

	// 推断合并：将 unknown outbound 归属到已知进程
	type targetKey struct {
		remAddr string
		remPort int
	}
	// 收集每个 target 被哪些已知进程连接
	targetOwners := make(map[targetKey]map[processKey]bool)
	unknownPK := processKey{"", 0}
	for key := range outboundCounts {
		if key.pk == unknownPK {
			continue
		}
		tk := targetKey{key.remAddr, key.remPort}
		if targetOwners[tk] == nil {
			targetOwners[tk] = make(map[processKey]bool)
		}
		targetOwners[tk][key.pk] = true
	}
	// 合并：unknown 的 outbound 如果 target 只有唯一已知进程，则合并
	for key, counts := range outboundCounts {
		if key.pk != unknownPK {
			continue
		}
		tk := targetKey{key.remAddr, key.remPort}
		owners := targetOwners[tk]
		if len(owners) != 1 {
			continue
		}
		// 取唯一的 owner
		var owner processKey
		for pk := range owners {
			owner = pk
		}
		// 合并 counts 到已知进程的 outbound 条目
		mergeKey := outboundKey{owner, key.remAddr, key.remPort}
		if outboundCounts[mergeKey] == nil {
			outboundCounts[mergeKey] = make(StateCounts)
		}
		for state, cnt := range counts {
			outboundCounts[mergeKey][state] += cnt
		}
		delete(outboundCounts, key)
	}

	// 构建进程分组
	type portRemotes struct {
		port    int
		remotes []RemoteStats
	}
	processData := make(map[processKey][]portRemotes)
	processOutbounds := make(map[processKey][]OutboundEntry)

	// 处理入站：按 (processKey, port) 汇聚 remotes
	type prKey struct {
		pk   processKey
		port int
	}
	prMap := make(map[prKey][]RemoteStats)
	for key, counts := range inboundCounts {
		total := 0
		for _, cnt := range counts {
			total += cnt
		}
		counts["TOTAL"] = total
		prk := prKey{key.pk, key.port}
		prMap[prk] = append(prMap[prk], RemoteStats{
			IP:     key.remIP,
			Counts: counts,
		})
	}
	for prk, remotes := range prMap {
		// 按 TOTAL 降序排列远程 IP
		sort.Slice(remotes, func(i, j int) bool {
			return remotes[i].Counts["TOTAL"] > remotes[j].Counts["TOTAL"]
		})
		processData[prk.pk] = append(processData[prk.pk], portRemotes{
			port:    prk.port,
			remotes: remotes,
		})
	}

	// 处理出站
	for key, counts := range outboundCounts {
		total := 0
		for _, cnt := range counts {
			total += cnt
		}
		counts["TOTAL"] = total
		processOutbounds[key.pk] = append(processOutbounds[key.pk], OutboundEntry{
			RemoteAddr: key.remAddr,
			RemotePort: key.remPort,
			Counts:     counts,
		})
	}

	// 收集所有出现过的进程键
	allKeys := make(map[processKey]bool)
	for pk := range processData {
		allKeys[pk] = true
	}
	for pk := range processOutbounds {
		allKeys[pk] = true
	}

	var groups []ProcessGroup
	for pk := range allKeys {
		pg := ProcessGroup{
			ProcessName: pk.name,
			PID:         pk.pid,
		}

		// Listeners: sorted by port ascending
		for _, pr := range processData[pk] {
			pg.Listeners = append(pg.Listeners, ListenerEntry{
				Port:    pr.port,
				Remotes: pr.remotes,
			})
		}
		sort.Slice(pg.Listeners, func(i, j int) bool {
			return pg.Listeners[i].Port < pg.Listeners[j].Port
		})

		// Outbounds: sorted by TOTAL descending
		pg.Outbounds = processOutbounds[pk]
		sort.Slice(pg.Outbounds, func(i, j int) bool {
			return pg.Outbounds[i].Counts["TOTAL"] > pg.Outbounds[j].Counts["TOTAL"]
		})

		groups = append(groups, pg)
	}

	// ProcessGroups: sorted by totalCount descending
	sort.Slice(groups, func(i, j int) bool {
		return totalCount(groups[i]) > totalCount(groups[j])
	})

	return groups
}
