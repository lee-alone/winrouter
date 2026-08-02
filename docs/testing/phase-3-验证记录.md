# Phase 3 验证记录

> 日期：2026-08-01  
> 状态：代码实现完成；退出验收进行中  
> 当前范围：HTTP CONNECT、域名引导解析、节点安全存储、订阅和代理 UI

## 已验证

| 任务 | 方法 | 结果 | 证据 |
|---|---|---|---|
| `P3-MODE-01` | 接受 `proxy-split`，继续拒绝 `hybrid` | 通过 | `TestValidateProxySplitRejectsMissingInvalidProxyAndHybrid` |
| `P3-MODE-02` | 断言生成配置使用 `route.final = proxy` | 通过 | `TestGenerateProxySplitUsesStrictProxyFinalBoundToInterfaceB` |
| `P3-MODE-03` | 缺少或无效代理时拒绝生成，无 B 直连回退 | 通过 | 同上模型测试 |
| `P3-NODE-01` | HTTP CONNECT IPv4/DNS 节点强类型生成及锁定核心校验 | 通过 | `tests/fixtures/config/proxy-split-http.json` |
| `P3-NODE-02` | 节点 CRUD、默认选择、损坏状态拒绝和原子持久化 | 通过 | `internal/nodes/store_test.go` |
| `P3-NODE-04` | Windows DPAPI 加解密往返，落盘内容不含密码原文 | 通过 | `TestDPAPIRoundTrip`、`TestStoreCRUDSelectionAndSecretRedaction` |
| `P3-NODE-03` | 对节点执行 HTTP CONNECT，记录可用性、状态码和延迟；Basic 认证头回归 | 通过 | `TestProbeHTTPConnectWithBasicAuthentication` |
| HTTP 认证配置 | 后端解密所选节点密码并注入核心配置，前端不接收原文 | 通过 | `ApplySelectedProxyConfiguration`、锁定 sing-box 检查 |
| `P3-UI-01` | 运行中禁止切换；进入代理模式要求已选节点和明确确认 | 通过 | Vue 类型检查与生产构建 |
| `P3-UI-02` | 节点列表、选择、测试状态、HTTP 状态与延迟展示 | 通过 | Vue 类型检查与生产构建 |
| `P3-UI-03` | 节点列表固定展示出口 B | 通过 | `frontend/src/App.vue`、Vue 类型检查与生产构建 |
| `P3-UI-05` | RPC 返回仅含 `has_password`，日志仅记录节点 ID/类型 | 通过 | `nodes.Node`、`app.go`、前端密码框 |
| `P3-PROXY-01/02` | 固定 UDP DNS 以网卡 B IPv4 为源地址解析代理域名 | 通过（自动化/模型） | `nodes.ResolveIPv4`、`App.interfaceBSourceIPv4` |
| `P3-PROXY-03` | proxy outbound 绑定 B，endpoint `/32` 强制 `foreign-direct`，缺失规则时语义校验失败 | 通过 | `TestGenerateProxySplitUsesStrictProxyFinalBoundToInterfaceB`、`TestProxySemanticsRejectMissingLoopPreventionRule` |
| `P3-PROXY-04` | 域名解析结果经 CONNECT 健康检查后原子提交，失败保留旧地址 | 通过 | `TestStoreCommitsResolvedIPAtomicallyAndPreservesItOnMetadataEdit` |
| `P3-SUB-01` | 版本化 JSON、未知字段/尾随值/未知协议拒绝，HTTPS、禁重定向、1 MiB 上限 | 通过 | `internal/subscriptions/manager_test.go` |
| `P3-SUB-02` | 订阅 URL 整体 DPAPI 加密；RPC/日志仅返回脱敏元数据 | 通过 | `TestRefreshStagesNodesAndRollsBackInvalidUpdate`、DPAPI 回归 |
| `P3-SUB-03` | 全批节点校验后原子替换；无效更新保留上一批节点 | 通过 | `TestReplaceSubscriptionIsAllOrNothing`、订阅刷新回归 |
| `P3-UI-04` | 代理/DNS/订阅失败与双层环路防护状态展示 | 通过 | Vue 类型检查与生产构建 |

## 当前限制

- 当前支持固定 IPv4 或 DNS 名称的 HTTP CONNECT 节点及可选 Basic 认证。
- 真实外部代理的双出口抓包、代理故障物理泄漏验证和 24 小时稳定性尚未执行。
- Phase 2 按用户决策跳过，保持阶段未通过记录，不阻塞 Phase 3 验收准备。

阶段结论：Phase 3 代码任务全部完成；真实外部代理出口验收和 24 小时稳定性尚未执行，
因此暂不满足 Phase 3 退出条件。
