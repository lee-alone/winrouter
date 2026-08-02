# Phase 2 验证记录

> 日期：2026-08-01  
> 状态：短周期验收完成；阶段未通过  
> 当前范围：稳定性恢复、配置迁移、双默认路由、开机启动、托盘、接口变化体验及短时资源基线

## 已验证

| 任务 | 方法 | 结果 | 证据 |
|---|---|---|---|
| `P2-STAB-05` | 恢复协调器故障注入测试 | 通过 | `internal/recovery/coordinator_test.go` |
| 配置重建 | GUID、名称、TUN 前缀、IPv4 直连前缀更新测试 | 通过 | `internal/recovery/config_test.go` |
| 用户停止竞态 | 等待恢复期间取消，确认不重新应用 | 通过 | `TestCoordinatorDoesNotRestoreAfterUserStop` |
| 有界失败 | 连续应用失败最多重试 3 次 | 通过 | `TestCoordinatorBoundsRecoveryRetries` |
| `P2-UX-03` | Wails bindings、Vue 类型检查及生产构建 | 通过 | `tools.ps1 -Task check`、`build/bin/WinRouter.exe` |
| `P2-STAB-07` 出口 A | 运行中物理断开/恢复以太网 | 通过 | sing-box/TUN 安全清理，自动恢复为 PID `8820`；`build/phase2-link-recovery/ethernet-a-20260801-024531.log` |
| `P2-STAB-07` 出口 B | 运行中断开/恢复 WLAN | 通过 | sing-box/TUN 安全清理，自动恢复为 PID `20380`；`build/phase2-link-recovery/wlan-b-20260801-024843.log` |
| `P2-STAB-01` 实机安全停止兜底 | 管理员脚本断开 WLAN，确认 TUN/核心清理并自动重连 | 通过 | `build/phase0/network-change.stdout.json`；脚本退出码 0；恢复后 WLAN/以太网均 Up、双默认路由 Alive |
| `P2-STAB-01` 自动化 | 所选出口地址/网关变化、默认路由丢失恢复、停止失败兜底与 TUN/metric 噪声注入 | 通过 | `TestCoordinatorRebuildsAfterSelectedAddressChanges`、`TestCoordinatorRebuildsAfterSelectedGatewayChanges`、`TestCoordinatorStopsWhenSelectedDefaultRouteDisappearsAndRestoresWhenItReturns`、`TestCoordinatorDoesNotApplyUntilSafeStopSucceeds`、`TestControllerRetainsFailedStopForRetry`、`TestCoordinatorIgnoresTUNAndMetricOnlyChanges` |
| `P2-STAB-01` DHCP 续租 | 运行中分别续租以太网与 WLAN | 通过 | 两次租约未变化，核心 PID `16096` 与 TUN 保持；`build/phase2-dhcp-recovery/before-20260801-052918.json`、`after-20260801-052918.json`、`renew-20260801-052918.log` |
| `P2-STAB-03` 自动化隔离 | 注入未选虚拟接口增删，确认不触发恢复；枚举当前机器虚拟/隧道接口分类 | 通过（自动化/当前环境） | `TestCoordinatorIgnoresUnselectedVirtualInterfaceChanges`；`build/phase2-compatibility/interfaces-20260801-053210.json`（37 个虚拟/隧道接口均 `candidate=false`） |
| `P2-STAB-06` | v1 状态迁移、无效迁移和未知版本故障注入 | 通过 | `internal/interfacemanager/store_test.go`，真实 `interfaces.json` 已迁移至 v2 |
| `P2-STAB-02` 休眠/唤醒 | 运行中由 Application API 进入睡眠并手动唤醒 | 通过 | 首轮复现运行时丢失；修复后 helper `22760`、sing-box `9744` 和 TUN 自动重建，TCP/DNS 通过 |
| `P2-UX-02` / 注销恢复 | 用户授权 HKCU 启动项，注销并重新登录 | 通过 | 自动启动 PID `18548`，A/B 保留；再次启动后 helper `23328`、sing-box `13948`，TCP 经 TUN 通过 |
| `P2-STAB-02` 系统重启 | 重启、重新登录、用户再次授权启动核心 | 通过 | 自动启动 PID `14480`；helper `8728`、sing-box `3520`、TUN、DNS 及经 TUN 的 TCP 全部通过 |
| `P2-STAB-04` | IPv4 网关族、A/B 默认路由归属和重叠私网故障注入 | 通过 | 双物理默认路由真实运行；`TestBuildCandidatesRequiresIPv4GatewayButAcceptsDualStackGateway`、`TestSelectedInterfacesRequireIndependentIPv4DefaultRoutes` 及既有 overlap 测试 |
| `P2-UX-01` | 关闭隐藏/恢复、菜单启动/停止、状态变化及显式退出真实交互 | 通过 | 启动后 helper `15860`、sing-box `10272`、TUN、DNS/TCP 正常；停止后 TUN 清理且直连恢复；修复版退出后全部进程和 TUN 立即清理 |
| helper 显式退出 | 认证 shutdown RPC、错误令牌拒绝及生产构建 | 通过 | `TestServerShutdownStopsServiceAndSignalsExit`、`TestServerRejectsUnauthenticatedShutdown`、`build/bin/WinRouter.exe` |
| `P2-UX-04` 连接与流量 | Windows IPv4 TCP/UDP 表、A/B 5 秒采样与 30 点趋势实机验证 | 通过 | 停止状态 34 活动 TCP、9 已建立、16 监听、30 UDP；运行状态趋势持续更新；`build/phase2-monitor-view.png` |
| `P2-UX-04` 规则命中 | 核心保留日志解析与国内/国外真实连接 | 通过 | B 命中 43→61；`1.0.1.1` 国内 CIDR 连接使 A 命中 0→2；`TestParseRuleHitsCountsConfirmedOutboundLines`、`build/phase2-monitor-a-hit.png` |
| `P2-UX-04` 验收退出 | 监控实测后从托盘显式退出并检查资源 | 通过 | WinRouter、helper、sing-box 和 TUN 均无残留；WLAN/以太网两条物理默认路由保持 Alive |
| `P2-UX-05` 诊断结果与建议 | 聚合接口诊断、核心/恢复状态和最近探测结果，验证建议动作不自动改网卡或绕过授权 | 通过 | `frontend/src/App.vue` 诊断评估区；`tools.cmd check`、Vue 类型检查及生产构建通过 |
| `P2-UX-06` 开发机基线 | 读取显示器/DPI 基线并检查 900px/680px 响应断点与窄窗口诊断区堆叠 | 部分完成 | 当前会话 120 DPI（约 125%），系统报告两个显示器设备；`frontend/src/style.css`；`tools.cmd check` 通过 |
| 当前环境复核 | Windows 构建、显示设备、DPI、网卡、默认路由及虚拟化相关进程快照 | 通过 | `build/phase2-short-validation/environment.json` |
| 锁定核心回归 | 当前拓扑分配 TUN 前缀并通过 sing-box 1.13.15 `check` | 通过 | `build/phase2-short-validation/topology.json`、`locked-core-check.json` |
| 诊断脱敏 | 密码、令牌、UUID、订阅凭据哨兵及固定文件清单 | 通过 | `build/phase2-short-validation/diagnostic-check-summary.json` |
| 30 秒空闲资源基线 | 生产程序 6 次采样，退出后检查 helper/core 残留 | 基线完成，非长稳结论 | CPU 累计约 0.33 秒；工作集 35.8→36.9 MiB；无 helper/sing-box 残留；`idle-resource-baseline.json` |

## 本次命令

```text
go test ./...
.\tools.cmd check
.\tools.cmd -Task build -Version 0.2.0-phase2-dev
```

结果：全部通过；npm audit 报告 0 个漏洞。

## 短周期验收结论

2026-08-01 已完成当前机器可执行且不依赖长时间窗口或额外硬件/软件环境的回归。
全量自动化、锁定核心配置检查、当前拓扑、诊断脱敏、生产构建和短时资源采样均通过。
30 秒采样仅建立基线：采样期间句柄仍处于初始化增长阶段，且测试策略尚未定义 CPU、
内存增长、吞吐、时延和丢包的正式阈值，因此不记为性能或泄漏验收通过。

| 延期项 | 原因 | 风险/后续计划 |
|---|---|---|
| 7 天连续运行 | 明确排除的长周期测试 | `R-006`；稳定版前定义阈值并执行 |
| Hyper-V/WSL/Docker/VPN 共存矩阵 | 当前没有活动组合，功能状态查询还需要管理员权限 | `R-004`；由对应环境回传脱敏诊断包 |
| 100/150/200% DPI、多显示器拖动 | 当前只有 120 DPI 与单活动 1920x1200 输出 | `R-005`；对应显示环境补测 |
| 真实 DHCP 地址/默认路由归属变化 | 当前续租前后租约不变 | 自动化故障注入已通过；跨 DHCP/路由环境补测 |

## 待执行

- 使用主动 TCP、UDP、DNS 探测和物理抓包确认恢复后没有错误出口泄漏。
- 默认路由实际变化及兼容性矩阵仍按 Phase 2 后续任务执行。
- 2026-08-01 05:27：管理员脚本断开 WLAN 后自动重连，WinRouter 探测器确认安全停止、TUN/路由清理和恢复后双出口探测通过；证据为 `build/phase0/network-change.stdout.json`。
- 2026-08-01 05:29：以太网与 WLAN 执行 `ipconfig /renew`，前后 IPv4 地址、网关和默认路由均保持；证据为 `build/phase2-dhcp-recovery/before-20260801-052918.json`、`after-20260801-052918.json` 和 `renew-20260801-052918.log`。
- DHCP 无变化续租已执行；本机未发生新地址分配，地址/网关/默认路由变化由自动化快照注入覆盖；跨 DHCP/路由环境兼容性纳入后续矩阵。
- Hyper-V、WSL、Docker 和常见 VPN 组合改由具备对应环境的用户测试，回传脱敏诊断包后按实际缺陷调试，不阻塞当前阶段。
- 本机已验证虚拟/隧道接口不会进入物理出口候选，且其变化不会触发恢复。决策标注：开发阶段不执行 Hyper-V、WSL、Docker 和常见 VPN 的真实共存矩阵；预览版由用户在实际环境测试并回传脱敏日志/诊断包，收到后再进行复现、判断和定向修复。
- 7 天连续运行测试尚未开始。
- `P2-UX-06` 决策标注：开发阶段不执行第二显示器、100/150/200% 缩放和跨显示器拖动矩阵；预览版由用户在实际环境验证并回传截图/日志，收到后再进行判断和定向修复。

## 复现缺陷

- 2026-08-01：首次真实运行发现界面把 IPv6 直连前缀加入 `ipv6=block` 配置，启动校验拒绝；已修复为只提交 IPv4 直连前缀。
- 2026-08-01：诊断页在空探测集合序列化为 `null` 时触发 Vue 渲染异常并显示空白；已在后端初始化空切片并在前端归一化 RPC 数据，重新构建及诊断导出回归通过。
- 2026-08-01：核心可启动并正确建立 TUN 默认路由，但使用期间用户网络不可用；路由与进程时间线保存于 `build/phase2-startup-outage/`，后由诊断包定位为全球 DNS 超时。
- 2026-08-01：诊断包确认启动断网由 UI 默认全局 DNS `1.1.1.1:853/DoT` 经出口 B 查询持续超时导致；已恢复为 Phase 1 验证过的 `8.8.8.8:53/UDP`，后续真实联网、休眠、注销、重启和双默认路由回归均通过。
- 2026-08-01：切换为 `8.8.8.8:53/UDP` 后用户确认启动期间联网恢复；体感速度偏慢，尚无量化吞吐、时延或丢包结论，纳入后续性能测量。
- 2026-08-01：首次休眠后主程序继续运行，但 helper、sing-box 与 TUN 消失，物理拓扑未改变导致恢复协调器未触发；已增加仅在期望运行状态启用的运行时存活监控。复测时主程序 PID `28588` 保持，helper 从 `25940` 重建为 `22760`，sing-box 从 `5396` 重建为 `9744`，TUN 接口索引从 `42` 更新为 `28`；`1.1.1.1:443` TCP 经 TUN 成功，`cloudflare.com` A 记录解析成功。
- 2026-08-01：托盘显式退出后主程序、sing-box 和 TUN 已清理，但 helper PID `15860` 仅能等待 20 秒空闲超时退出并锁住构建产物；根因为客户端关闭只停止心跳。已新增经会话令牌认证的最小 `shutdown` RPC，先停止核心再关闭 helper 监听器，并保留空闲超时兜底；授权和拒绝回归、完整检查、生产构建及真实退出复测全部通过。
- 2026-08-01：首次规则命中实测持续显示 0，原因是解析器沿用了 Phase 0 实验标签 `a-direct/b-direct`，而当前 MVP 生成器使用 `domestic-direct/foreign-direct`。解析器已绑定当前标签并兼容旧证据格式；修复版实测 A/B 命中均按真实连接增长。

阶段结论：Phase 2 当前环境短周期验收完成，但阶段不通过；`P2-STAB-01/02/04/05/06/07`
与 `P2-UX-01/02/03/04/05` 已通过。兼容性矩阵、跨环境 DHCP/默认路由变化、
高 DPI/多显示器、7 天稳定性和正式性能阈值验证尚未完成，因此不满足 Phase 2 全部退出条件。
