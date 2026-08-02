# Phase 1 验证记录

> 状态：实现验收完成；公开发布条件未满足  
> 开始日期：2026-07-31  
> 当前构建：0.1.0-dev

## 当前范围

本记录随 Phase 1 实施持续更新。当前覆盖工程骨架、InterfaceManager、
PolicyEngine/ConfigGenerator、提权 helper/核心控制、MVP 用户界面以及监控、
日志与诊断。
Phase 1 最终网络出口验收已经完成。当前记录覆盖功能、接口、配置、权限、安全、
恢复、性能风险豁免和已执行兼容性；第二套 Windows 与活动虚拟网络组合仍未执行，
作为公开发布条件保留，不伪造覆盖结论。

## 环境

- Windows 11 x64，Go 模块版本 1.25.0（本次执行工具链 Go 1.26.1）。
- Node.js 24.14.0，npm 固定基线 11.9.0（允许同主版本兼容执行器）。
- Wails 2.13.0，sing-box 1.13.15。
- Vue 3.5.21、TypeScript 5.9.2、Vite 7.3.6、vue-tsc 3.0.6。

## 执行结果

| 用例 | 任务 | 命令/检查 | 结果 | 证据 |
|---|---|---|---|---|
| P1-ENG-T01 | P1-ENG-01/02 | Vue 类型检查和生产构建 | 通过 | `frontend/package-lock.json`、`frontend/dist/` |
| P1-ENG-T02 | P1-ENG-03 | `gofmt`、`go vet ./...`、`go test ./...` | 通过 | `tools.cmd` 控制台结果 |
| P1-ENG-T03 | P1-ENG-03 | `npm audit` | 通过，0 个漏洞 | `npm ci` 审计结果 |
| P1-ENG-T04 | P1-ENG-04 | 接口、配置、规则固定输入存在 | 通过 | `tests/fixtures/` |
| P1-ENG-T05 | P1-ENG-05 | Wails Windows/amd64 生产构建 | 通过 | `build/bin/WinRouter.exe`、`build/metadata/` |
| P1-IF-T01 | P1-IF-01/04 | Windows 枚举、候选过滤和确定性推荐 | 通过 | `internal/interfaces/`、`internal/interfacemanager/` 测试 |
| P1-IF-T02 | P1-IF-02 | GUID 优先、辅助匹配、原子状态持久化 | 通过 | `manager_test.go`、`store_test.go` |
| P1-IF-T03 | P1-IF-03 | 状态和默认路由变化事件 | 通过 | `TestManagerRunPublishesNetworkChange`、`TestManagerRunPublishesDefaultRouteChange` |
| P1-IF-T04 | P1-IF-05 | 直连前缀与跨接口重叠诊断 | 通过 | `topology_test.go`、`TestManagerReportsOverlapAndPoolExhaustion` |
| P1-IF-T05 | P1-IF-06 | TUN 前缀确定分配、复用、持久化、冲突复核 | 通过 | `tunprefix` 与 `interfacemanager` 测试 |
| P1-IF-T06 | P1-IF-07 | 接口消失、GUID 变化、重叠、地址池耗尽 | 通过 | `go test -count=1 ./internal/interfacemanager ./internal/interfaces ./internal/tunprefix` |
| P1-IF-T07 | P1-IF-01..07 | 全量检查与 Wails Windows/amd64 构建 | 通过 | `tools.cmd`、`build/bin/WinRouter.exe` |
| P1-CFG-T01 | P1-CFG-01/02 | v1 Schema、强类型模型、标准库 JSON 序列化 | 通过 | `docs/schema/winrouter-config-v1.schema.json`、`mvp_generator_test.go` |
| P1-CFG-T02 | P1-CFG-03..06 | 策略分类顺序与最终 `route.rules` 一致、输出确定 | 通过 | `direct_split_test.go`、`TestGenerateMVPMapsPolicyOrderToFinalJSON`、`TestGenerateMVPIsDeterministic` |
| P1-CFG-T03 | P1-CFG-07 | 双 DNS 出口、固定 IP、防环路和独立缓存 | 通过 | `TestGenerateMVPMapsPolicyOrderToFinalJSON`、锁定核心 check |
| P1-CFG-T04 | P1-CFG-08 | AAAA 和 IPv6 路由显式拒绝 | 通过 | `TestGenerateMVPMapsPolicyOrderToFinalJSON`、锁定核心 check |
| P1-CFG-T05 | P1-CFG-09 | 模型、Schema、语义、sing-box 四层校验 | 通过 | `TestValidationLayersRejectTamperedOutput`、`TestGeneratedMVPPassesLockedSingBoxCheck` |
| P1-CFG-T06 | P1-CFG-10 | IPv4/IPv6、保留地址、重叠前缀、错误模式与枚举 | 通过 | `go test -count=3 ./internal/policy ./internal/config` |
| P1-CFG-T07 | P1-CFG-01..10 | 全量检查与 Windows/amd64 构建 | 通过 | `tools.cmd`、`build/bin/WinRouter.exe` |
| P1-CORE-T01 | P1-CORE-01/02 | 普通 UI、按需 UAC helper、heartbeat 看门狗 | 通过 | `helper-core.md`、提权烟测；会话结束后 helper/core/TUN 均为 0 |
| P1-CORE-T02 | P1-CORE-03/04/10 | SID ACL、随机令牌、长度帧、方法与参数白名单 | 通过 | `server_test.go`、Windows Named Pipe 集成测试 |
| P1-CORE-T03 | P1-CORE-05 | 核心版本、SHA-256 和 `sing-box check` | 通过 | `core.Validate`、提权烟测 |
| P1-CORE-T04 | P1-CORE-06 | 原子提交、TUN 健康检查、候选失败回滚 | 通过 | `controller_test.go`、提权烟测 |
| P1-CORE-T05 | P1-CORE-07 | Job Object、CTRL_BREAK 与强杀回退 | 通过 | Phase 0 Job Object 证据复用、生产启动器、提权烟测 |
| P1-CORE-T06 | P1-CORE-08 | 1/2/4 秒有界退避、最多 3 次、异常标记 | 通过 | `TestControllerRestartsWithBoundedBackoff`、`TestControllerStopsAfterRestartLimit` |
| P1-CORE-T07 | P1-CORE-09 | 仅清理固定候选/回滚文件和受控进程资源 | 通过 | `TestControllerRestoresOwnedStateOnly`、提权烟测 |
| P1-CORE-T08 | P1-CORE-01..10 | 10 轮控制器/RPC 回归、全量构建 | 通过 | `go test -count=10 ./internal/core ./internal/helperipc`、`tools.cmd` |
| P1-UI-T01 | P1-UI-01/03 | 首次选择、同接口互斥、详情、策略和诊断呈现 | 通过 | `frontend/src/App.vue`、Vue 类型检查 |
| P1-UI-T02 | P1-UI-02 | 用户选择、核心应用、出口验证、速率采集状态、告警与 IPv6 状态分层呈现 | 通过 | `frontend/src/App.vue` |
| P1-UI-T03 | P1-UI-04 | 预检、启动、停止、异常恢复和网卡变更确认 | 通过 | Wails 绑定类型检查、`tools.cmd` 生产构建 |
| P1-UI-T04 | P1-UI-05 | 接口事件及操作日志、三级过滤、错误关联 ID、200 条上限 | 通过 | `frontend/src/App.vue` |
| P1-UI-T05 | P1-UI-06 | 原生键盘控件、焦点可见、840px 最小窗口、680/900px 响应断点、长文本换行、强制高对比度 | 通过（静态与构建检查） | `frontend/src/style.css`、Vue 类型检查 |
| P1-UI-T06 | P1-UI-01..06 | 全量检查与 Windows/amd64 生产构建 | 通过 | `tools.cmd`、`build/bin/WinRouter.exe` |
| P1-OBS-T01 | P1-OBS-01 | 500 条有界结构化应用日志、256 KiB 有界核心输出收集 | 通过 | `internal/observability/logger.go`、`core.Status.core_log` |
| P1-OBS-T02 | P1-OBS-02/03 | TCP、NTP/UDP、DNS 探测模型及源地址到接口 GUID 核对 | 通过（实现与单测）；真实双出口待验收 | `probe.go`、`TestInterfaceForSourceUsesEnumeratedAddress` |
| P1-OBS-T03 | P1-OBS-04 | Windows `GetIfEntry2` 接口收发字节采样 | 通过，本机实际枚举读取 | `TestReadInterfaceCountersFromWindows` |
| P1-OBS-T04 | P1-OBS-05/06 | 诊断 ZIP 预览、原子导出及固定文件清单；无 rule-set 二进制 | 通过 | `TestWriteBundleRedactsSecretsAndExcludesRuleSetBinary` |
| P1-OBS-T05 | P1-OBS-07 | 字段、自由文本、UUID、Bearer 和 URL 查询参数脱敏 | 通过 | `observability_test.go` |
| P1-OBS-T06 | P1-OBS-08 | rule-set 版本、SHA-256、来源、条数、大小和加载结果 | 通过 | `RuleSetMetadata`、应用成功记录 |
| P1-OBS-T07 | P1-OBS-01..08 | Vue 类型检查、前端生产构建、`go vet`、全量 Go 测试 | 通过 | `tools.cmd check` |
| P1-EXIT-T01 | Phase 1 退出 | 实际 MVP/helper 单轮应用、核心日志、停止与清理 | 通过 | `WinRouter-core-smoke.exe`；running PID 21120，停止后 TUN 清理 |
| P1-EXIT-T02 | Phase 1 退出 | 实际 MVP/helper 100 轮应用/停止/逐轮 TUN 清理 | 通过，100/100；清理等待 8–133 ms | `WinRouter-core-smoke.exe --cycles 100` 控制台 JSON |
| P1-EXIT-T03 | Phase 1 退出 | TCP、NTP/UDP 双出口主动探测及 `pktmon` 交叉确认 | 通过 | `build/phase1/dual-outbound.*` |
| P1-EXIT-T04 | Phase 1 退出 | 双 DNS 解析与物理源地址抓包 | 通过（定向配置）；最终 MVP 语义待复测 | `build/phase1/dual-dns.*` |
| P1-EXIT-T05 | Phase 1 退出 | 两侧 LAN ICMP 可达及各自物理接口抓包 | 通过 | `build/phase1/dual-lan.*` |
| P1-EXIT-T06 | Phase 1 退出 | AAAA 与 IPv6 TCP 阻断、停止后资源清理 | 通过 | `build/phase1/ipv6-block.*` |
| P1-EXIT-T07 | Phase 1 退出 | 同一份生产 MVP 配置下验证国内/其他公网业务和双 DNS 的语义路由、物理出口及反向泄漏 | 通过；9/9 断言 | `build/phase1-mvp-semantic/mvp-semantic-summary.json`、`.etl`、`.capture.txt` |
| P1-EXIT-T08 | Phase 1 退出 | 实际诊断 ZIP 的固定清单、可解压性、JSON 完整性、敏感哨兵泄漏扫描、二进制排除与人工内容抽查 | 通过 | `build/phase1-diagnostics/diagnostic-check-summary.json`、`diagnostic-sentinel.zip` |
| P1-EXIT-T09 | Phase 1 退出 | 生产 helper 强制终止其持有的核心，验证有界重启、异常标记与最终资源清理 | 通过；1 秒退避后第 1 次重启成功，TUN 清理 124 ms | `build/phase1-core-crash/core-crash-production.json` |

标准复现命令：

```powershell
.\tools.cmd
```

## 风险与结论

当前目录未暴露可用 Git 仓库元数据，因此本地开发构建的 `commit` 字段为
`unknown`；CI 或正常 Git 工作树会写入实际短 SHA。该限制不影响工程骨架
验证，但正式发布构建不得使用 `unknown`。

接口变化监听采用 1 秒只读快照指纹；短于一个轮询周期且完全恢复的瞬态变化
可能不产生 UI 事件。Go 竞态检测因当前工具链未启用 CGO而无法执行
`go test -race`；常规单元测试、`go vet` 和 Wails 构建均通过。

工程骨架、InterfaceManager、PolicyEngine 与 ConfigGenerator、提权 helper、
核心控制、MVP 用户界面和监控诊断工作通过。受当前自动化环境限制，本轮未取得真实
WebView2 在 125%/150% Windows 缩放下的截图证据；响应式、高对比度、键盘和
长文本项目已完成静态与生产构建检查，真实缩放外观仍列入发布前桌面兼容性
复测。Phase 1 实现验收完成，公开发布条件仍未满足。

2026-07-31 补充：当前双物理出口环境完成 100 轮实际 MVP/helper 生命周期，
以及 TCP、UDP、DNS、LAN、IPv6 四组提权探测和 `pktmon` 抓包。国内 A、其他
公网 B 和 DNS 三项仍需使用最终 MVP 配置再做一次语义级抓包，因此未勾选。

2026-08-01 补充：最终 MVP 配置语义抓包通过。同一份由 `GenerateMVP` 生成的
`direct-split` 配置中，国内业务/DNS 仅命中 WLAN A，其他公网业务/全球 DNS
仅命中以太网 B；错误接口反向断言均为零命中，两个域名解析成功，核心日志的
`domestic-direct`/`foreign-direct` 与物理源地址一致，停止后 TUN/路由清理。
ETL SHA-256 为 `2CAFBF6D79557CB6B0613ABA7F3B7E2A17DB0B9D27F3D468B74E9FFBA5A3721C`，
文本抓包 SHA-256 为 `B028A1122CA19088158B80EFCA77AD6C6911FBDA57154845003170E59936D4BB`。
24 小时性能测试按 D-003 在预览版阶段明确豁免，未执行且不构成 CPU、内存或
丢包阈值通过证据。物理网卡掉线、
重连、自动重建和实际休眠恢复已按 D-002 延后至 Phase 2，不阻断 MVP。

## 2026-08-01 严格降级预验收

- 核心强杀基线：实验核心被强制结束后，无重启子进程、sing-box 残留、TUN
  地址或 TUN 路由；证据 `build/phase0/core-crash.result.json`。该脚本使用
  Phase 0 实验控制器，不能替代生产 helper 的有界重启验收。
- 网卡断开基线：WLAN 禁用后 3.519 秒内检测到变化，核心正常停止，TUN 接口
  与路由均清理；证据 `build/phase0/network-change.stdout.json`。
- 恢复结果：原脚本未能在 60 秒内确认 WLAN 恢复并完成双出口重建，因此整项
  失败。随后单独恢复成功，WLAN 为 Up，地址为 `10.12.85.4/24`；证据
  `build/phase0/wlan-recovery-status.json` 与 `wlan-recovery-address.json`。
- 范围结论：`App.Startup` 收到 `interfaces:changed` 后目前只向 UI 发事件并记录
  日志，前端也只刷新快照；没有调用生产 helper 停止核心。根据 D-002，该能力及
  物理网卡掉线、重连、自动重建、实际休眠恢复统一转入 Phase 2 的
  `P2-STAB-02/05/07`，不再阻断 Phase 1。以上失败结果作为后续优化基线保留，
  Phase 1 仅继续完成生产 helper 的核心崩溃有界处理验收。

## 2026-08-01 诊断包功能与脱敏验收

- 工具：`cmd/winrouter-diagnostic-check`。生成包含密码、Token、API key、Bearer、
  内联密码、订阅凭据、URL credential 和 UUID 哨兵的实际 ZIP，再重新打开检查。
- 结果：8/8 固定条目均存在且为合法 JSON；全部敏感哨兵零明文命中，脱敏标记
  存在；只包含 `rule-sets.json` 元数据，不包含完整 rule-set 二进制。
- 人工抽查：应用状态、核心状态、探测目标、接口计数器、规则版本/哈希/条数仍
  可用，敏感字段和 UUID 被替换为 `[REDACTED]`，不是整文件清空。
- 证据：`build/phase1-diagnostics/diagnostic-check-summary.json`、
  `diagnostic-sentinel.zip`。ZIP SHA-256：
  `184478D7B86237AAED7BDF65B16C820C57BB3DDBC6F96AEFDA1B02BDA542ADD5`；汇总
  SHA-256：`0B107F1BE99E33629B5BC6F6CCEBE19BCF76B94AEEE922BE12D92219B4E8F6B6`。

## 2026-08-01 生产核心崩溃验收

- 工具：`WinRouter-core-smoke.exe --crash-test`。烟测通过 UAC 启动生产 helper；
  helper 仅在显式测试开关下接受无 PID、无路径参数的受限核心终止请求。
- 结果：初始核心 PID `30368` 被终止，控制器记录异常退出，并按 1 秒退避在
  第 1 次尝试重启为 PID `24048`；`restart_attempts = 1`、`abnormal = true`。
- 清理：重启实例随后通过正常停止路径退出，TUN 接口在 124 ms 内消失，无残留。
- 证据：`build/phase1-core-crash/core-crash-production.json`。

## 2026-08-01 预览版长稳测试豁免

- 决策：Phase 1 仅作为预览版发布，不执行 24 小时连续运行测试；该项是风险
  豁免，不记为性能测试通过，正式稳定版前重新评估受控长稳验证。
- 反馈：运行异常时保留本地有界日志，由用户主动导出脱敏诊断包；默认不上传，
  后续按应用版本、错误关联 ID、发生时间和复现频率归类分析。
- 发布边界：出现流量泄漏、TUN 或路由无法清理、无限重启、持续崩溃等严重问题
  时停止预览版发布并优先修复，不等待更多用户样本。
- 已知缺口：当前没有 24 小时 CPU、内存和丢包阈值数据，不能据此声明长期稳定性。

## Phase 1 自审与结论

- 安全与代码自审：通过已实现范围；普通 helper 默认关闭验收故障注入，RPC、
  路径、核心哈希、资源所有权和诊断脱敏边界无新增阻断或严重缺陷。
- 自动检查：`tools.cmd check`、`go mod verify` 和生产构建通过；npm 生产依赖审计
  为 0 漏洞。详细结果见 `docs/release/phase-1-self-review.md`。
- 实现结论：Phase 1 `direct-split` MVP 在当前 Windows 11 双网卡环境验收通过，
  阻断级和严重级产品缺陷为 0，可以进入 Phase 2 开发。
- 发布结论：`0.1.0-preview` 仅为本地预览候选。项目许可证、签名、可追踪 Git
  commit、第二套干净 Windows 安装/升级/卸载和兼容测试未完成，因此尚不可称为
  公开或商业发布就绪。
