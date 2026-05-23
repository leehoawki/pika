# pika

Linux 服务器网络连接可视化工具。一条命令看清所有进程的网络连接全貌。

## 截图示意

在一台运行 nginx + redis 的服务器上执行 `sudo pika`：

```
nginx (PID: 1234)
  ├─ :80 (IN)
  │   ├─ 10.0.0.3 (TOTAL:120 ESTABLISHED:45 TIME_WAIT:75)
  │   ├─ 10.0.0.5 (TOTAL:32 ESTABLISHED:30 TIME_WAIT:2)
  │   └─ 10.0.0.8 (TOTAL:5 ESTABLISHED:5)
  ├─ :443 (IN)
  │   ├─ 10.0.0.3 (TOTAL:88 ESTABLISHED:60 TIME_WAIT:28)
  │   └─ 10.0.0.12 (TOTAL:15 ESTABLISHED:15)
  └─ 127.0.0.1:6379 (OUT) (TOTAL:4 ESTABLISHED:4)

redis-server (PID: 5678)
  └─ :6379 (IN)
      ├─ 127.0.0.1 (TOTAL:4 ESTABLISHED:4)
      └─ 10.0.1.5 (TOTAL:2 ESTABLISHED:2)

sshd (PID: 910)
  ├─ :22 (IN)
  │   └─ 192.168.1.100 (TOTAL:1 ESTABLISHED:1)
  └─ :2222 (IN)
      └─ 88.0.8.244 (TOTAL:1 ESTABLISHED:1)
```

一眼就能看到：

- **nginx** 是流量大户，监听了 80 和 443 端口接收外部请求，同时作为 Redis 客户端连着本地 6379
- **redis-server** 监听 6379，被 nginx 和另一台机器 10.0.1.5 连着
- **sshd** 监听 22 和 2222 端口，多个 SSH 会话自动合并为一个分组（即使底层是不同的 sshd-session 子进程）

## 安装

```bash
go build -o pika .
sudo cp pika /usr/local/bin/
```

交叉编译：

```bash
GOOS=linux GOARCH=amd64 go build -o pika-linux-amd64 .
GOOS=linux GOARCH=arm64 go build -o pika-linux-arm64 .
```

## 使用

```bash
sudo pika                        # 查看所有进程的网络连接
sudo pika --process nginx        # 只看 nginx 的连接
sudo pika --process redis        # 只看 redis 的连接（子串匹配，redis-server 也会匹配）
```

> 需要 `sudo` 才能看到进程信息，否则所有连接会归入 `(unknown)`。

## 输出解读

### 树形结构

```
进程名 (PID: 进程号)
  ├─ :端口 (IN)              ← 该进程监听的端口，有外部连接进来
  │   ├─ 远程IP (状态计数)
  │   └─ 远程IP (状态计数)
  └─ 远程IP:端口 (OUT) (状态计数)  ← 该进程主动发起的外部连接
```

- **IN** — 入站连接。该进程监听端口上接收的外部连接，按远程 IP 聚合
- **OUT** — 出站连接。该进程主动向外部发起的连接，按目标 IP:端口 聚合

### 排序

- 进程按总连接数从多到少排列，热点进程一目了然
- 进程内 IN 在前、OUT 在后
- IN 按端口号升序，OUT 按连接数降序
- 同一端口下的远程 IP 按连接数降序

### 状态计数

`TOTAL:N ESTABLISHED:X TIME_WAIT:Y ...`

- **TOTAL** — 总连接数
- **ESTABLISHED** — 活跃连接
- **TIME_WAIT** — 等待关闭的连接（大量堆积说明短连接频繁）
- **CLOSE_WAIT** — 对端关闭但本端未关（堆积说明程序有 bug）
- **SYN_SENT / SYN_RECV** — 握手中的连接

## 典型使用场景

### 排查连接泄露

```
redis-server (PID: 5678)
  └─ :6379 (IN)
      └─ 127.0.0.1 (TOTAL:500 ESTABLISHED:500)
```

500 个 ESTABLISHED 到同一个 IP？检查应用是否正确关闭了 Redis 连接池。

### 发现异常外连

```
mysqld (PID: 3306)
  ├─ :3306 (IN)
  │   └─ 10.0.1.5 (TOTAL:3 ESTABLISHED:3)
  └─ 45.33.32.156:443 (OUT) (TOTAL:1 ESTABLISHED:1)
```

mysqld 不应该有对外的 HTTPS 连接，可能是入侵。

### 快速评估负载

```
nginx (PID: 1234)
  └─ :80 (IN)
      ├─ 10.0.0.3 (TOTAL:120 ESTABLISHED:45 TIME_WAIT:75)
      └─ 10.0.0.5 (TOTAL:2 ESTABLISHED:2)
```

TIME_WAIT:75 占比很高，考虑开启 `tcp_tw_reuse` 或改用长连接。

## 核心推理逻辑

pika 并不是简单地把 `netstat` 输出做个格式化，而是做了两层智能推理来让输出结果更准确、更有意义。

### 父子进程合并（`main.go:resolveParentConnections`）

**问题场景：** 很多服务进程（如 sshd、nginx）采用 master-worker 模式——父进程负责 LISTEN，子进程/worker 负责处理具体连接。`netstat` 输出中这些连接会分散在不同的子进程 PID 下，导致同一个服务的连接碎片化，难以整体观察。

例如 `sshd` (PID 100) 监听端口 22，每次 SSH 登录会 fork 出 `sshd-session` (PID 200, PPID=100) 处理连接。`netstat` 会显示 LISTEN 归 PID 100，ESTABLISHED 归 PID 200，看起来像两个不相关的进程。

**解决方式：** pika 通过 `ps -ef` 获取完整进程树（PID → PPID 映射），然后按以下规则合并：

```
对每条连接：
  1. 如果是 LISTEN 状态 → 只用 ps -ef 清洗进程名，绝不改变 PID
     （LISTEN 是进程的"身份"，不能移动）

  2. 如果是非 LISTEN 状态（ESTABLISHED、TIME_WAIT 等）：
     a. 查找该连接进程的父进程（通过 PPID）
     b. 检查父进程是否在同一个本地端口上有 LISTEN
     c. 如果是 → 将连接归属到父进程（修改 PID 和进程名）
     d. 如果否 → 保持原样
```

**关键的防过度合并设计：** 合并只在"父进程在同一端口有 LISTEN"时才发生。这意味着：

- `sshd-session` (PPID=sshd) 的端口 22 连接 → 合并到 `sshd`（因为 sshd LISTEN 22）✓
- `nginx worker` (PPID=nginx) 的端口 80 连接 → 合并到 `nginx`（因为 nginx LISTEN 80）✓
- `nginx` (PPID=systemd) 的端口 80 连接 → **不会**合并到 `systemd`（systemd 没有 LISTEN 80）✗

如果没有这个限制，所有进程最终会被递归合并到 PID 1 (systemd)，结果就没有意义了。

> `ps -ef` 是可选的。如果执行失败（如容器内没有 ps 命令），pika 退化为直接使用 netstat 的原始进程信息，不做任何合并。

### 未知进程推断（`model/connection.go:Classify`）

**问题场景：** 不加 `sudo` 运行 `netstat -tnap` 时，其他用户的进程连接的 PID 列会显示 `-`，pika 将其解析为 PID=0、进程名为空的"未知连接"。这些连接通常是已关闭进程遗留的 TIME_WAIT 或 CLOSE_WAIT。

**推断算法：**

```
1. 按 (remoteAddr, remotePort) 对所有出站连接进行分组
2. 对每个目标地址，统计有哪些已知进程连向它
3. 对每条未知的出站连接：
   - 查看它的目标地址 (remoteAddr:remotePort)
   - 如果该目标只被唯一一个已知进程连接过 → 合并到该进程
   - 如果有多个已知进程连向同一目标 → 保持 (unknown)，不做猜测
```

**示例：**

```
已知连接：
  nginx (PID 1234) → ESTABLISHED → 10.0.0.100:3306
  nginx (PID 1234) → ESTABLISHED → 10.0.0.100:3306

未知连接：
  (unknown)        → TIME_WAIT   → 10.0.0.100:3306
  (unknown)        → TIME_WAIT   → 10.0.0.100:3306

只有 nginx 连向 10.0.0.100:3306 → 推断 TIME_WAIT 也属于 nginx → 合并
```

```
已知连接：
  nginx   (PID 1234) → ESTABLISHED → 10.0.0.100:3306
  php-fpm (PID 5678) → ESTABLISHED → 10.0.0.100:3306

未知连接：
  (unknown)          → TIME_WAIT   → 10.0.0.100:3306

nginx 和 php-fpm 都连向 10.0.0.100:3306 → 无法确定归属 → 保持 (unknown)
```

这个推断是保守的——宁可留下 `(unknown)` 也不做错误的猜测。加 `sudo` 运行可以完全避免这个问题。

## 依赖

- Linux 系统，需要 `netstat` 命令（通常包含在 `net-tools` 包中）
- `ps` 命令（用于进程父子关系解析，通常已预装；缺失时不影响基本功能）
- Go 1.22+（编译时）
- 零第三方依赖，纯标准库实现

## 参数

| 参数 | 说明 |
|------|------|
| `--process NAME` | 按进程名过滤（子串匹配，不区分大小写） |
| `-h, --help` | 显示帮助 |
| `-v, --version` | 显示版本号 |

## 项目结构

```
pika/
├── main.go              # 入口，CLI 参数解析，父子进程合并
├── netstat/parser.go    # 执行 netstat 并解析输出
├── process/parser.go    # 执行 ps -ef 并解析进程树
├── model/connection.go  # 数据结构、按进程分组与聚合
├── filter/filter.go     # 按进程名过滤
├── chart/tree.go        # 连接树渲染
└── chart/color.go       # ANSI 颜色常量
```

## License

MIT
