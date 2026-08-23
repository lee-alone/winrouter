# 虚拟出口 C 与统一多出口路由改造计划

## 1. 改造目标

将当前以 `direct-split` / `proxy-split` 为中心的配置模型，演进为统一的多出口策略模型：

- 出口 A：绑定物理网卡 A 的直连出口
- 出口 B：绑定物理网卡 B 的直连出口
- 出口 C：当前选中的代理节点出口，底层连接绑定 A 或 B
- `reject`：显式拒绝
- `final`：可配置的默认出口，支持 A、B、C

必须保持现有双网卡路由、防代理环路、DNS 防泄漏、IPv6 策略和启动前语义校验能力。

## 2. 设计决策

### 2.1 直接切换到统一模型

内测阶段不保留旧版本兼容逻辑，不实现 v1 到 v2 的迁移，也不同时维护两套配置语义。可以直接更新现有 schema 的字段语义和校验规则；旧配置由用户删除或重新生成。

出口决策统一由 `default_outbound` 与规则动作决定。动作枚举统一为 `a`、`b`、`c`、`reject`；`final` 表示使用默认出口。若 `mode` 不再提供独立语义，应直接删除，避免继续保留无效的模式分支。

### 2.2 出口解析集中化

新增统一的出口解析函数，将 `a/b/c` 映射为 `domestic-direct`、`foreign-direct`、`proxy`。custom rule、rule-set、`route.final`、DNS final 和规则预览必须使用同一套解析逻辑。

## 3. 分阶段实施

### 阶段 0：冻结新模型基线和测试夹具

范围：`internal/config`、`internal/policy`、`tests/fixtures`。

1. 定义统一 A/B/C/reject/final 配置样例。
2. 增加 A/B/C/reject/final 决策矩阵测试。
3. 删除或改写依赖 `direct-split` / `proxy-split` 的旧 fixture。
4. 记录 `go test ./...` 基线。

完成标准：新配置模型和所有出口组合均有明确预期；不再承诺旧配置可读取。

### 阶段 1：配置模型与持久化改造

范围：`internal/config/mvp_model.go`、`mvp_generator.go`、`mvp_validate.go`、`internal/rulesettings/manager.go`、`app.go`。

1. 直接修改现有配置结构和 schema 版本约束，使其表达统一出口模型。
2. `default_outbound` 支持 `a/b/c`。
3. custom rule 和 rule-set action 支持 `c`。
4. 删除旧 `mode` 分支及其兼容解析代码；如果保留字段，仅作为无语义的临时 UI 字段。
5. C 无有效节点时明确报错，禁止静默降级。
6. 更新规则设置存储、默认值和读写测试。

完成标准：统一模型可保存、重新加载，非法 C 配置在模型阶段被拒绝。

### 阶段 2：统一路由生成和语义校验

范围：`internal/policy/direct_split.go`、`internal/config/mvp_generator.go`、`internal/config/mvp_validate.go`。

1. custom rule 和 rule-set 的 `c` 映射为 `proxy`。
2. `route.final` 由 `default_outbound` 统一决定。
3. proxy outbound 仅在存在有效节点时生成。
4. 保证 route rule、preview、category 顺序一致。
5. 语义校验覆盖 final 引用、C 配置完整性、代理 endpoint 防环路和禁止隐式回退。

完成标准：A/B/C/reject/final 组合均有正向和反向校验，生成 JSON 可通过 sing-box 校验。

### 阶段 3：DNS 出口联动

范围：`internal/config`、`internal/nodes`、`app.go`。

1. 新增 `dns.proxy` 或由生成器创建 `dns-proxy`。
2. `dns-domestic`、`dns-global`、`dns-proxy` 分别 detour 到 A、B、C。
3. C 规则和默认 C 使用 `dns-proxy`。
4. proxy endpoint 的 bootstrap DNS 必须通过底层 A/B，不能经由 C。
5. C 不可用时默认拒绝，不回退到 A/B。
6. 增加 DNS 泄漏、污染和 proxy DNS 防环路测试。

完成标准：业务连接和 DNS 使用一致的出口意图，proxy endpoint 不进入 proxy outbound。

### 阶段 4：节点选择、健康状态和恢复

范围：`app.go`、`internal/nodes`、`internal/recovery`、`internal/observability`。

1. 当前选中节点明确作为出口 C 实例。
2. 节点切换时重新解析、健康检查并生成核心配置。
3. 节点失效时暂停 C 相关流量并报告原因，不自动降级。
4. 记录节点 ID、底层 egress、解析 IP、健康状态和失败原因。
5. 重新解析后替换旧 endpoint 防环路规则。

完成标准：节点切换、失效、恢复和重启后，C 状态与核心配置一致。

### 阶段 5：前端和 API

范围：`frontend/src/App.vue`、`i18n.ts`、Wails 类型和配置 API。

1. 出口选择统一显示 A、B、C、Reject。
2. 默认出口支持 A/B/C。
3. 节点界面显示 C 的底层 egress A/B。
4. 展示用户选择、核心已应用、出口已验证三种状态。
5. C 无节点、健康检查失败、DNS 失败时显示阻断原因。
6. 规则预览显示实际 outbound tag。

完成标准：前端可以创建、预览和应用 C 配置，错误状态可解释。

### 阶段 6：端到端验收

| 场景 | 预期结果 |
| --- | --- |
| 规则 A | 业务连接和 DNS 均走网卡 A |
| 规则 B | 业务连接和 DNS 均走网卡 B |
| 规则 C | 业务走 proxy，proxy endpoint 走底层 A/B |
| 默认 C | 未命中规则的流量走 proxy |
| C 无节点 | 应用失败或相关流量明确拒绝 |
| endpoint 变化 | 旧防环路规则删除，新规则生效 |
| IPv6 block | AAAA 和 IPv6 流量被拒绝 |
| IPv6 split | A/B/C 的 IPv6 路径和 DNS 策略一致 |
| 启停循环 | 无残留进程、TUN、路由或旧配置 |
| 配置重建 | 新配置可保存、加载并保持出口语义 |

建议执行：

```text
go test ./...
go test -race ./internal/config ./internal/policy ./internal/recovery
```

Windows 环境还应运行现有 elevated 脚本，并新增 A/B/C 代理出口脚本。

## 4. 风险与控制措施

| 风险 | 控制措施 |
| --- | --- |
| C 不可用时意外直连 | C 缺失或失效时拒绝，不做隐式降级 |
| 代理环路 | endpoint 直连规则、生成后语义校验、端到端验证 |
| DNS 泄漏 | DNS detour 与业务出口一致，专项探测 |
| 配置模型切换 | 明确标记 breaking change，启动时拒绝旧格式 |
| 私有网段冲突 | 复用现有接口拓扑和 TUN 前缀预检 |
| UI 与核心不一致 | 同时读取核心状态和主动出口探测 |

## 5. 提交边界

1. `test: add unified outbound decision matrix`
2. `config: switch to unified outbound model`
3. `policy: support outbound C and unified final`
4. `dns: bind proxy resolver to outbound C`
5. `app: refresh selected proxy and recovery semantics`
6. `ui: expose unified outbound selector and state`
7. `test: add end-to-end multi-egress acceptance`

每个提交都应保持 `go test ./...` 通过；阶段 3 之后才允许开启 C 作为用户可选出口。由于不保留旧配置兼容，配置模型切换提交应明确标记为 breaking change。

## 6. 完成定义

1. 统一模型读写和所有出口组合均有自动化测试。
2. A/B/C/reject/final 在路由和 DNS 中语义一致。
3. C 的代理 endpoint 永不再次命中 C。
4. C 失效时不会静默直连。
5. IPv4、IPv6 block、IPv6 split 均通过验收。
6. UI 状态与核心配置和主动探测一致。
7. 启停、网络变化、节点切换和恢复无残留资源。
