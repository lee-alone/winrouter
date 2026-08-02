# Phase 0 验证记录

## 当前结论

- 状态：当前单环境通过
- 日期：2026-07-31
- 环境：Windows 11 专业版 10.0.26200 x64
- sing-box：1.13.15

Phase 0 在当前 Windows 11 双网卡环境完成接口、TUN、TCP/UDP、内网、DNS、IPv6、代理防环路和生命周期验证。自动化测试 47/47 通过，5/5 Go 包通过，`go vet ./...` 通过。第二 Windows 环境和真实外部代理节点物理抓包未执行，列为 Phase 1 发布前兼容性风险，不影响当前独立开发进入下一阶段。

## 环境摘要

- 设备：Acer Shadow SH16-41，15.3 GB RAM。
- WLAN：Intel Wi-Fi 6E AX210，驱动 23.40.0.4。
- 以太网：Realtek PCIe GbE，驱动 1168.19.704.2024。
- Go：1.26.1 windows/amd64。
- 核心 SHA-256：`4DB8218DEA131668CCD5E0B32E773E916A37BE730E655B176E0D3A930276CBE7`。
- 详细策略与环境矩阵：`docs/testing/测试策略与环境.md`。

## 缺陷与遗留风险

- 已修复：DNS 缺少 sniff 导致递归日志增长；TUN 出现后探测过早；DNS 探针被 hijack；IPv6 仅 TCP Dial 误判；UDP 无响应未使实验失败；WLAN 重新启用但未自动关联。
- 已观察：Cloudflare 加密 DNS 在 WLAN 首次可能超时，重试成功；后续应实现健康检查和降级。
- 未覆盖：第二 Windows 环境、活动 Hyper-V/WSL/Docker/VPN 组合、真实外部代理节点物理抓包。
- 验收决定：以上为兼容性覆盖风险，不是当前单环境的阻断缺陷。

## 自审结论

- 网络职责：规则顺序、DNS 防环路、IPv6 阻断、物理出口和清理证据已复核。
- Windows 平台职责：接口稳定标识、Job Object、CTRL_BREAK、网卡变化和 S0 恢复已复核。
- 测试职责：关键用例、失败迭代、原始证据、47 项单元测试和静态检查已复核。

## P0-IF 双网卡基线

### 环境拓扑

| 测试标签 | 接口 | GUID | 索引 | IPv4 | 网关 | 状态 |
|---|---|---|---:|---|---|---|
| A | WLAN | `{25C2CF5B-09E1-4B5C-81BB-222E2BF9DB5E}` | 6 | `10.12.85.4/24` | `10.12.85.232` | up |
| B | 以太网 | `{C594B97D-EBFC-4698-B39E-22FACCF82F86}` | 16 | `192.168.1.166/24` | `192.168.1.1` | up |

本轮 A/B 仅为测试标签，两张网卡均可完整访问网络，不用于判断国内或全球访问能力。

### P0-IF-BASE-01 各接口网关可达

- 前置条件：两块接口均为 up。
- 步骤：分别使用对应源地址向各自网关发送 3 个 ICMP Echo。
- 预期：两组丢包率均为 0%，且目标为各自网关。
- 结果：通过。
  - A：`10.12.85.4 -> 10.12.85.232`，0% 丢包，平均 8 ms。
  - B：`192.168.1.166 -> 192.168.1.1`，0% 丢包，平均 1 ms。

### P0-IF-BASE-02 受约束路由决策

- 前置条件：以管理员令牌执行只读路由诊断。
- 步骤：以 `1.1.1.1` 为目标，分别约束两个源地址执行 Windows 路由诊断。
- 预期：源地址 A/B 分别选择其对应接口和网关。
- 结果：通过。
  - `10.12.85.4` -> 接口 6 -> `10.12.85.232` -> `0.0.0.0/0`。
  - `192.168.1.166` -> 接口 16 -> `192.168.1.1` -> `0.0.0.0/0`。
  - 两条默认路由状态均为 Alive。
- 原始证据：`build/phase0/dual-interface-baseline.json`。

### 注意事项

两次绑定本地地址访问公网回显服务得到相同公网 IP。该结果可能来自共同上游 NAT，不单独用于证明出口接口；接口选择以 Windows 受约束路由诊断和后续抓包为准。

## P0-TUN-01 最小 TUN

详细记录见 `docs/experiments/P0-TUN-01-minimal-tun.md`，单环境 `system` stack 创建、地址配置和强制停止清理已通过。

## P0-TUN-03 双 direct outbound

### P0-TUN-DIRECT-01 固定目标命中指定 outbound

- 日期：2026-07-31
- 配置：A outbound 绑定 `WLAN`，B outbound 绑定`以太网`。
- 固定规则：`1.1.1.1/32 -> a-direct`，`8.8.8.8/32 -> b-direct`。
- DNS 上游通过 `b-direct`，避免 DNS 请求与 A 的固定测试规则混淆。
- 前置校验：配置通过 sing-box 1.13.15 `check`，核心版本和 SHA-256 匹配锁定清单。
- 结果：通过配置层、TUN 层、核心路由层和物理接口抓包交叉验证。
  - 两个 TCP 探针的本地地址均为 `172.19.0.1`，确认流量进入 TUN。
  - `1.1.1.1:443` 连接成功，核心日志显示 `outbound/direct[a-direct]`。
  - `8.8.8.8:443` 连接成功，核心日志显示 `outbound/direct[b-direct]`。
  - `162.159.200.1:123` NTP 请求收到 48 字节有效响应，核心日志显示 `outbound/direct[a-direct]`。
  - `162.159.200.123:123` NTP 请求收到 48 字节有效响应，核心日志显示 `outbound/direct[b-direct]`。
  - `pktmon` 在 Wi-Fi 组件 11 捕获 A 的 TCP/UDP 发包，物理源地址为 `10.12.85.4`，目标分别为 `1.1.1.1:443` 和 `162.159.200.1:123`。
  - `pktmon` 在 Ethernet 组件 12 捕获 B 的 TCP/UDP 发包，物理源地址为 `192.168.1.166`，目标分别为 `8.8.8.8:443` 和 `162.159.200.123:123`。
  - 停止后 TUN 地址已清理，相关路由为 0，无残留核心或探针进程。
- 原始证据：
  - `build/phase0/dual-outbound-captured.stdout.json`
  - `build/phase0/dual-outbound-captured.stderr.log`
  - `build/phase0/dual-outbound-captured.etl`
  - `build/phase0/dual-outbound-captured.capture.txt`

### 实验器修正记录

首次实验在 TUN 接口出现后立即探测，早于 `auto_route` 稳定，两个连接绕过 TUN 并使用 `192.168.1.166`。实验器增加 1.5 秒路由稳定等待后复测，两个连接均使用 TUN 地址并分别命中预期 outbound。

首次 UDP 尝试使用 DNS 目标，流量被通用 `hijack-dns` 规则接管，不能证明 A/B UDP outbound。随后改用 NTP 请求；一个固定目标未响应但日志确认路由正确，最终换用两个 Cloudflare NTP anycast 地址后，A/B 均取得有效响应。

## P0-TUN-04 内网分流

### P0-TUN-LAN-01 两个直连前缀保持各自接口可达

- 日期：2026-07-31
- 配置规则：`10.12.85.0/24 -> a-direct (WLAN)`，`192.168.1.0/24 -> b-direct (以太网)`。
- 步骤：在双 outbound TUN 活跃且路由稳定后，分别向 `10.12.85.232` 和 `192.168.1.1` 发送两个 ICMP Echo，同时使用 `pktmon` 仅在物理 NIC 层抓包。
- 结果：通过。
  - A 网关收到 2/2 响应，0% 丢包，平均 16 ms；请求和响应仅在 Wi-Fi 组件 11 出现，源地址为 `10.12.85.4`。
  - B 网关收到 2/2 响应，0% 丢包，平均 1 ms；请求和响应仅在 Ethernet 组件 12（及其下层组件 92）出现，源地址为 `192.168.1.166`。
  - 两组抓包均未出现在另一物理出口。
  - Windows 的现有直连 `/24` 路由比 TUN 自动路由更具体，因此网关探测按系统直连路由发出，不进入 sing-box TUN；配置中的显式前缀规则用于被 TUN 接管场景，本用例验证 TUN 活跃时直连局域网未被破坏或误导向另一出口。
  - 停止后 TUN 地址和相关路由均已清理，无残留 `sing-box` 或探针进程。
- 原始证据：
  - `build/phase0/dual-lan.stdout.json`
  - `build/phase0/dual-lan.stderr.log`
  - `build/phase0/dual-lan.etl`
  - `build/phase0/dual-lan.capture.txt`

## 本轮执行方式

管理员会话执行 `build/run-final-network-tests-elevated.ps1`，依次运行 `--run-dual` 与 `--run-lan`，每次实验均单独启动和停止 `pktmon`，并在结束时移除过滤器。

## P0-DNS-01 双出口 DNS

### P0-DNS-SPLIT-01 UDP 上游绑定与缓存隔离

- 日期：2026-07-31
- 配置：`dns-a=1.1.1.1:53 -> a-direct (WLAN)`，`dns-b=8.8.8.8:53 -> b-direct (以太网)`；启用 `independent_cache`，`route.default_domain_resolver=dns-b`。
- 规则：`one.one.one.one -> dns-a`，`dns.google -> dns-b`，其余查询默认 `dns-b`。
- 结果：已覆盖 UDP 双上游，测试通过。
  - `one.one.one.one` 返回 `1.1.1.1`、`1.0.0.1`；核心日志显示 `a-direct -> 1.1.1.1:53`。
  - `dns.google` 返回 `8.8.8.8`、`8.8.4.4`；核心使用 `dns-b`，抓包显示 `8.8.8.8:53` 从以太网发出。
  - `pktmon` 捕获 A 上游源地址 `10.12.85.4`、Wi-Fi 组件 11；B 上游源地址 `192.168.1.166`、Ethernet 组件 12。
  - 上游均使用 IP 地址，默认域名解析器显式指定，5.6 秒实验核心日志约 7.8 KB，未出现递归增长。
  - 停止后 TUN 地址、路由和相关进程均无残留。
- 原始证据：`build/phase0/dual-dns.stdout.json`、`dual-dns.stderr.log`、`dual-dns.etl`、`dual-dns.capture.txt`。
- 后续小节补充 DoH/DoT 引导解析及真实 IP/fake-IP 对照结果。

### P0-DNS-SPLIT-02 DoT/DoH 引导解析防环路

- 日期：2026-07-31
- 最终组合：`dns-a=DoT 1.1.1.1:853 -> WLAN`，`dns-b=DoH 8.8.8.8:443/dns-query -> 以太网`；TLS SNI 分别为 `cloudflare-dns.com` 和 `dns.google`。
- 结果：通过。
  - 两个上游均使用固定 IP 引导，不依赖自身域名解析；`default_domain_resolver=dns-b` 明确设置。
  - `one.one.one.one` 和 `dns.google` 均返回预期 A 记录。
  - `pktmon` 显示 DoT 源地址为 `10.12.85.4`、Wi-Fi 组件 11；DoH 源地址为 `192.168.1.166`、Ethernet 组件 12。
  - 16.2 秒实验核心日志约 23.8 KB，无递归连接或无界增长；停止后零残留。
  - 兼容性现象：Cloudflare DoH 经 WLAN 查询超时，交换为 Cloudflare DoT 后首次查询仍在 10 秒超时一次，但自动重试约 1.3 秒成功；Google DoH 正常。该现象不属于环路，但后续健康检查应记录上游延迟并支持降级。
- 原始证据：`build/phase0/encrypted-dns.stdout.json`、`encrypted-dns.stderr.log`、`encrypted-dns.etl`、`encrypted-dns.capture.txt`。

## P0-DNS-02 DNS 地址模型

### P0-DNS-MODEL-01 真实 IP 与 fake-IP 对照

- 日期：2026-07-31
- 真实 IP：`example.com` 返回 `104.20.23.154`、`172.66.147.243`；TCP 从 TUN 地址连接公网地址成功。
- fake-IP：`example.com` 返回 `198.18.0.2`；TCP 连接该地址成功，核心日志确认恢复域名并以 `b-direct` 连接 `example.com:80`。
- 两次实验均通过锁定核心检查，停止后 TUN 地址、路由和相关进程无残留。
- MVP 选择：真实 IP。
  - 理由：更接近 Windows 和应用默认语义，不占用 `198.18.0.0/15`，降低 VPN、虚拟网络、地址字面量和不兼容应用的风险。
  - 限制：域名规则必须在 DNS 阶段可靠分类；CDN 地址复用场景不能只依赖解析后的 IP 延续域名策略。
  - 回退：fake-IP 作为后续可选能力保留，启用前必须完成地址冲突检测、VPN/Hyper-V/WSL 共存及典型应用兼容测试。
- 原始证据：`build/phase0/realip.stdout.json`、`realip.stderr.log`、`fakeip.stdout.json`、`fakeip.stderr.log`。

## P0-IPV6-01 MVP IPv6 策略

### P0-IPV6-BLOCK-01 活动期间阻断、停止后恢复

- 日期：2026-07-31
- 选择：MVP 明确阻止 IPv6，不做静默直连降级。TUN 增加 `fdfe:dcba:9876::1/126`，DNS 使用 `ipv4_only`，DNS 劫持规则之后设置 `ip_version=6 -> reject`。
- 结果：通过。
  - 活动期间查询 `one.one.one.one` 的 IPv6 地址返回 `no such host`，没有 AAAA 地址。
  - 到 `[2606:4700:4700::1111]:443` 的连接进入 TUN；TLS 握手被强制关闭，未建立到真实远端的 TLS 会话。
  - 停止后系统原生查询 `cloudflare.com AAAA` 立即返回两个 IPv6 地址，证明未永久修改系统 DNS/IPv6 状态。
  - 停止后 TUN 地址、路由和相关进程均无残留。
- 原始证据：`build/phase0/ipv6-block.stdout.json`、`ipv6-block.stderr.log`、`ipv6-post-stop-aaaa.json`。
- 探针修正：仅以 TCP `Dial` 成功不足以判断泄漏，因为用户态 TUN 可先完成本地握手；最终使用 TLS 握手是否完成作为判据。

## P0-PROXY 代理防环路

### P0-PROXY-LOOP-01 受控单节点 HTTP CONNECT

- 日期：2026-07-31
- 节点：受控 HTTP CONNECT 服务监听 `192.168.1.166:18080`，域名为 `proxy.winrouter.test`。
- DNS：独立 `hosts` DNS server `proxy-bootstrap` 将节点域名解析到网卡 B 地址；proxy outbound 的 `domain_resolver` 仅指向该 server。
- 防环路规则顺序：节点域名和节点 IP `/32 -> b-direct`，测试目标 `1.1.1.1/32 -> proxy`，其余最终出口为 `b-direct`。
- 结果：通过受控单机实验。
  - `1.1.1.1:443` 探针从 `172.19.0.1` 进入 TUN，核心日志恰有一条 `outbound/http[proxy]`。
  - 受控代理只收到 1 次 CONNECT；4.9 秒运行期间没有重复代理连接或资源增长。
  - proxy outbound 显式 `bind_interface=以太网`，节点 IP 同时由 endpoint `/32` 规则固定到 `b-direct`。
  - 停止后 TUN 地址、路由、核心和探针进程无残留。
  - 节点 IP 更新单元测试将地址改为 `192.168.1.200`，确认 bootstrap DNS 和 `/32` 同步替换，旧 `192.168.1.166/32` 不残留。
- 原始证据：`build/phase0/proxy-loop.stdout.json`、`proxy-loop.stderr.log`。
- 范围限制：本轮节点与客户端在同一 Windows 主机，证明配置路由、指定解析和递归抑制；真实外部代理节点经网卡 B 的物理抓包仍应在后续集成环境复测。

## P0-LIFE 生命周期

### P0-LIFE-01 正常停止与 50 次循环

- 日期：2026-07-31
- 实现：核心使用独立 Windows 进程组；停止时发送 `CTRL_BREAK`，等待超时才允许 `Kill` 兜底，并在报告中区分停止方式。
- 结果：通过。
  - 单次实验确认 `stop_method=ctrl_break`、`graceful_stop=true`。
  - 连续 50 次均完成：50/50 优雅停止，50/50 TUN 地址清理，50/50 路由清理，无 fallback kill。
  - 耗时：最小 1984 ms，平均 3238.1 ms，P95 3457 ms，最大 3467 ms。
  - 循环外检查：相关进程、TUN 地址和路由均为 0。
- 原始证据：`build/phase0/lifecycle-50.stdout.json`、`lifecycle-50.stderr.log`。

### P0-LIFE-02 控制进程强制结束

- 实现：sing-box 启动后立即加入设置 `JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE` 的 Windows Job Object。
- 故障注入：TUN 建立后强制结束控制探针 PID 20852。
- 结果：Job Object 自动终止核心 PID 18072；TUN 地址和路由均为 0，无残留进程。
- 原始证据：`build/phase0/parent-kill.result.json`、`parent-kill.child.stderr.log`。

### P0-LIFE-03 核心崩溃

- 故障注入：TUN 建立后强制结束核心 PID 17008。
- 结果：控制器报告核心异常退出并停止；等待观察期间重启子进程为 0，剩余核心、TUN 地址和路由均为 0。
- 当前 Phase 0 策略：不自动重启，因此不存在无限重启；Phase 1 若增加重启，必须使用有上限的指数退避。
- 原始证据：`build/phase0/core-crash.result.json`、`core-crash.child.stderr.log`。

### P0-LIFE-04 网卡断开与恢复

- 故障注入：双出口 TUN 活跃时短时禁用 `WLAN`，以太网保持在线。
- 结果：通过严格降级与恢复重建验证。
  - 监测器识别 `changed_interface=WLAN`，约 3.7 秒内使用 `CTRL_BREAK` 优雅停止并清理 TUN/路由。
  - WLAN 使用原有 `iQOO Neo8 Pro` 配置恢复，地址回到 `10.12.85.4/24`，网关 `10.12.85.232`。
  - 重新枚举和生成配置后，A/B TCP 2/2、UDP 2/2 通过，随后再次优雅停止并清理。
- 原始证据：`build/phase0/network-change.stdout.json`、`network-change.stderr.log`、`network-recovery.stdout.json`、`network-recovery.stderr.log`。
- 脚本修正：首次恢复只启用适配器但未在 30 秒内重新关联 SSID；增加显式使用原有 WLAN profile 连接及 60 秒 DHCP 等待后复测通过。

### P0-LIFE-05 休眠恢复

- 日期：2026-07-31
- 环境睡眠类型：Windows S0 低功耗待机（Connected Standby）。
- 步骤：双出口 TUN 活跃后调用系统睡眠，人工唤醒；控制器通过定时器时间跳变识别恢复事件，执行严格降级安全停止，再重新枚举接口和启动双出口验证。
- 结果：通过。
  - 检测到真实睡眠/恢复时间跳变，唤醒后 4772 ms 内完成 `CTRL_BREAK` 优雅停止及 TUN/路由清理，低于 30 秒要求。
  - WLAN 恢复为 `10.12.85.4/24`。
  - 恢复后重建策略，TCP 2/2、UDP 2/2 通过，再次优雅停止并清理。
  - 最终相关进程、TUN 地址和路由均为 0。
- 原始证据：`build/phase0/sleep-resume.stdout.json`、`sleep-resume.stderr.log`、`sleep-resume.metadata.json`、`sleep-recovery.stdout.json`、`sleep-recovery.stderr.log`。
- 实验器修正：首次恢复探测出现单个 NTP 无响应但未使进程失败，发现 UDP 判定缺口；现已将任何 UDP 无响应纳入整体失败条件，修正后复测 2/2 通过。

## 后续用例

1. 在外部代理测试环境复测节点物理出口。
2. 在第二套 Windows 环境执行兼容性复测。
