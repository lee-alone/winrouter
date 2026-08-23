# WinRouter IPv6 分流功能落地修改指南

本文用于指导在现有 `block`/`split` 双模式基础上逐步完善 IPv6 数据流分流。目标是让每个阶段都可独立编译、测试和回滚，避免在 IPv6、DNS、代理和 UI 同时变更时出现不可定位的回归。

## 1. 当前兼容契约

首批实现继续保持现有行为：

- `block` 是默认策略。AAAA 查询和 TUN 内 IPv6 流量均拒绝。
- `split` 是严格双栈策略。出口 A、B 都必须有可用 IPv6 全局地址和 IPv6 默认路由。
- `split` 不做静默单栈降级。任一出口不满足条件时，预检失败并阻止应用配置。
- DNS 策略暂时固定为 `block -> ipv4_only`、`split -> prefer_ipv4`。
- IPv6 TUN 地址暂时仍由生成器提供，但必须经过冲突检测；不能因为开启 IPv6 而绕过已有安全规则。

单栈自动降级、`prefer_ipv6`/`ipv6_only` 用户配置和 IPv6 GeoIP 全量规则属于后续独立需求，不应与第一批修复混合提交。

## 2. 第一批：接口索引与预检

### 2.1 修复默认路由匹配

修改 `internal/interfacemanager/manager.go` 的 `hasIPv6DefaultRoute`：

1. 仅匹配规范化前缀 `::/0`。
2. 首选比较 `route.InterfaceLUID == adapter.LUID`（当两者非零）。
3. LUID 不可用时比较 `route.InterfaceIndex == adapter.IPv6Index`。
4. 不要用 IPv4 `adapter.Index` 作为 IPv6 路由的唯一判断条件。

建议抽出一个可测试的 `routeBelongsToIPv6Adapter` helper，并覆盖：IPv4/IPv6 索引不同、LUID 相同、LUID 缺失、索引为零四种情况。

### 2.2 保持严格预检

继续使用 `hasUsableIPv6`，但确认以下地址不应被视为出口地址：未指定、回环、链路本地和组播地址。预检诊断必须指出具体角色（A/B）及失败原因：缺少全局地址或缺少默认路由。

验收命令：

```powershell
go test ./internal/interfacemanager ./internal/interfaces ./internal/routes
go test ./...
```

## 3. 第二批：IPv6 代理节点端到端支持

### 3.1 配置校验

修改 `internal/config/mvp_generator.go` 的代理地址校验：

- 接受合法的全局 IPv4 或 IPv6 单播地址。
- 仍拒绝未指定、回环、链路本地、组播地址。
- 错误信息改为“usable fixed IP address”，不要再写“IPv4”。

同步检查节点模型、JSON 编解码、前端表单和配置迁移，确保 IPv6 文本不会被拆分或重新格式化。

### 3.2 Bootstrap 解析

统一调用 `nodes.ResolveEndpoint`，不要在新路径继续调用仅支持 IPv4 DNS/source 的 `ResolveIPv4`。分两步完成：

1. 先允许通过 IPv4 DNS 查询 AAAA，保证 IPv6 代理服务器可解析。
2. 再扩展 DNS server/source 的地址族支持；IPv6 DNS 上游必须使用 IPv6 本地源地址，不能把 IPv4 `LocalAddr` 复用于 IPv6 连接。

DNS 上游应保留固定地址，避免 Bootstrap 自身依赖待解析的域名。

### 3.3 探测与连接

`internal/nodes/probe.go` 已使用 `net.JoinHostPort`，且 `dialerForEndpoint` 已避免 IPv4 source 绑定到 IPv6 remote。补充测试验证：

- IPv6 literal 节点 TCP 握手；
- IPv6 节点使用 IPv4 source 时不设置错误的 `LocalAddr`；
- HTTP CONNECT/TLS 探测使用方括号地址格式；
- 解析出的 IPv6 节点可生成有效 sing-box 出站。

## 4. 第三批：TUN IPv6 前缀与安全规则

### 4.1 TUN 前缀

`internal/tunprefix/allocator.go` 已包含 `fc00::/7` 冲突检测，不能重复实现一套检测逻辑。应做以下改造：

- 将 IPv6 TUN 前缀从 `mvp_generator.go` 硬编码提取为模型字段或版本化默认值；
- 让分配器同时检查接口地址和路由表中的 IPv6 前缀；
- 保留冲突来源、接口索引和现有前缀，供 UI 诊断；
- 配置迁移时为旧配置填充默认 IPv6 前缀。

### 4.2 保留段规则

在 `internal/policy/direct_split.go` 中分开处理“禁止外发段”和“必须直连段”，不要把所有 IPv6 特殊段合并为无条件 reject：

- 可直接保护：`::/128`、`::1/128`、`fe80::/10`、`ff00::/8`、`2001:db8::/32`；
- `fc00::/7` 必须排除当前 TUN 前缀及用户声明的本地直连前缀；
- 物理网卡 on-link 前缀应在 direct-prefix 规则中先于保留段规则匹配。

规则顺序必须保持：metadata/local/infrastructure -> user -> direct-prefix -> reserved -> domestic。

## 5. 第四批：DNS 防环路与 IPv6 规则输入

### 5.1 DNS 上游防环路

确认 `hostPrefix` 和 `normalizePrefixes` 能将 IPv4 地址转为 `/32`、IPv6 地址转为 `/128`。对国内/全球 DNS 上游分别生成对应 detour 规则，并测试 IPv6 上游不会回到 TUN 默认路由。

### 5.2 自定义 IPv6 CIDR

`internal/rulesettings` 和 `policy.normalizePrefixes` 应统一使用 `net/netip`：

- 接受单个 IPv6 地址和 CIDR；
- 规范化为 masked 前缀；
- 去重时按规范化前缀和 action 判断；
- 拒绝非法、未指定、回环或组播地址（除非规则类型明确允许）；
- 保持 IPv4 现有排序和冲突行为不变。

为 IPv4/IPv6 各增加单元测试，并加入混合地址排序测试。

## 6. 暂缓项目及启动条件

以下功能暂不进入首批实现：

- 单栈出口自动降级；
- 用户可选 `prefer_ipv6`、`ipv4_only`、`ipv6_only`；
- 完整中国 IPv6 GeoIP/SRS 规则集；
- 睡眠唤醒后的真实 SLAAC/DHCPv6 重路由；
- UI 全量 IPv6 状态改造；
- 双独立 IPv6 出口的真实抓包验收。

启动这些项目之前，必须先具备：稳定的 IPv6 出口状态模型、DNS 地址族契约、失败时的回滚行为和至少一台可重复测试的双栈 Windows 环境。

## 7. 提交与回滚要求

建议拆成以下独立提交：

1. `fix: match IPv6 default route by LUID/IPv6 index`；
2. `feat: accept IPv6 proxy endpoints`；
3. `feat: make IPv6 TUN prefix configurable and diagnosable`；
4. `fix: protect IPv6 infrastructure and reserved ranges`；
5. `test: cover IPv6 DNS, rules, proxy and probe paths`。

每个提交都必须通过 `go test ./...`；涉及前端时再执行 `npm run build`。配置生成失败时必须保留上一份健康配置，不能提交部分生成结果。任何真实机验收失败，都应先关闭 `split` 回到 `block`，再定位问题。

## 8. 完成定义

第一阶段只有在以下条件全部满足后才可标记完成：

- IPv6 默认路由在 IPv4/IPv6 索引不同的 Windows 适配器上能正确识别；
- IPv6 literal 代理节点可通过校验、生成、解析和 TCP 探测；
- IPv6 DNS 上游有 `/128` 防环路规则；
- TUN IPv6 前缀冲突可被检测并给出可操作诊断；
- `block` 行为未改变，`split` 不再残留 IPv6 reject 规则；
- 全量 Go 测试和配置语义校验通过。

## 9. 审查发现的技术边界与补充建议

### 9.1 Windows IPv6 路由索引兼容性

不同 Windows 版本以及不同路由 API 返回路径中，`routes.Route.InterfaceIndex` 可能表现为 IPv4 `IfIndex`，也可能表现为 IPv6 `Ipv6IfIndex`。因此不能只比较 `adapter.IPv6Index`。

建议使用以下判定顺序：

```go
func routeBelongsToIPv6Adapter(route routes.Route, adapter interfaces.Adapter) bool {
    if route.InterfaceLUID != 0 && adapter.LUID != 0 {
        return route.InterfaceLUID == adapter.LUID
    }
    return route.InterfaceIndex == adapter.IPv6Index || route.InterfaceIndex == adapter.Index
}
```

该 helper 只应被 `hasIPv6DefaultRoute` 使用，并配套测试 LUID 有效、LUID 缺失、IPv4/IPv6 索引相同及不同四类输入。若 LUID 均存在但不相等，不应再使用 Index 回退，以免误把另一网卡的默认路由判给当前网卡。

### 9.2 Bootstrap 解析的完整调用链

修改 `internal/nodes/resolve.go` 不足以完成节点 IPv6 支持。`app.go` 中 `withSelectedProxy` 以及节点测速/刷新路径仍可能直接调用 `nodes.ResolveIPv4` 并传入 IPv4 source。

实施时应：

1. 全局搜索 `ResolveIPv4`，逐个确认是否属于代理 Bootstrap 调用；
2. 在 `withSelectedProxy` 中改用 `ResolveEndpoint`；
3. source address 取实际出站接口的可用地址，IPv4 DNS 使用 IPv4 source，IPv6 DNS 使用 IPv6 source；
4. 保留 `ResolveIPv4` 仅用于明确要求“只解析 A 记录”的旧兼容路径；
5. 增加调用链测试，确保节点域名解析失败不会静默回退到未解析的域名地址。

### 9.3 First-Match 规则与 ULA 处理

当前策略生成顺序是：

`Metadata/Local/Infrastructure -> User -> Direct Prefix -> Reserved -> Domestic`

sing-box 使用 first-match 语义时，物理网卡的 on-link IPv6 前缀如果已生成到 `Direct Prefix`，会在 `Reserved` 之前命中 `route`，因此不需要为了保护局域网而把所有 ULA 从保留段中删除。

可以将 `fc00::/7`、`fe80::/10`、`ff00::/8` 放入 `CategoryReserved` 的 reject 规则，但必须确认：

- TUN 自身 ULA 前缀不会被这条规则错误拦截；
- TUN 地址和物理 on-link 前缀在生成阶段具有更高优先级的本地/直连处理；
- direct-prefix 与 reserved 发生重叠时，预览结果明确显示实际 first-match 规则。

不要仅凭“规则排在前面”推断所有 TUN 控制流量都安全，仍需对 TUN 地址、DNS 劫持和回环路径做生成配置测试。

### 9.4 IPv6 单 IP 输入的兼容解析

用户规则输入应同时接受单 IP 和 CIDR。`internal/rulesettings/manager.go` 的 `normalizeValue` 不应只调用 `netip.ParsePrefix`，建议使用以下语义：

```go
if addr, err := netip.ParseAddr(value); err == nil {
    value = netip.PrefixFrom(addr, addr.BitLen()).String()
} else if prefix, err := netip.ParsePrefix(value); err == nil {
    value = prefix.Masked().String()
} else {
    return "", errors.New("invalid IP or CIDR")
}
```

IPv4 单 IP 应自动变为 `/32`，IPv6 单 IP 应自动变为 `/128`；已有 CIDR 则必须 masked 后再去重、排序和持久化。错误信息应同时适用于 IPv4 和 IPv6，避免 UI 误报为“仅支持 IPv4”。
