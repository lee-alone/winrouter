# 配置与策略契约

## 输入模型

配置继续使用 `schema_version = 1`，正式 Schema 位于
`docs/schema/winrouter-config-v1.schema.json`，固定 fixture 位于
`tests/fixtures/config/direct-split.json`。

开放模式为 `direct-split` 和 `proxy-split`，`hybrid` 在模型校验阶段直接拒绝。
`proxy-split` 当前首批协议为 HTTP CONNECT；应用节点可提供固定 IPv4 或 DNS 名称，
进入核心模型前必须解析为固定 IPv4 和端口；
生成器唯一设置 `route.final = proxy`，代理 outbound 显式绑定网卡 B。缺少或
无效节点时拒绝生成配置，不回退到网卡 B 直连。A/B 必须具有不同 GUID 和
`bind_interface`；DNS 上游必须是固定 IP，MVP IPv6 策略唯一允许值为 `block`。

节点可使用固定 IPv4 或 DNS 名称。DNS 名称通过配置中的固定全局 UDP DNS，
以网卡 B 的 IPv4 为源地址解析；新地址通过 CONNECT 健康检查后才原子提交。
生成配置同时使用网卡 B outbound 绑定和 endpoint `/32` 直连规则防止代理环路。
节点密码使用 Windows DPAPI 加密持久化，并仅在 Go 后端解密后注入 helper 配置，
前端 RPC 不返回密码原文。当前支持 HTTP Basic 认证。

## 规则顺序

`internal/policy.BuildDirectSplit` 生成分类规则，ConfigGenerator 按原顺序
逐项映射到 `route.rules`：

Phase 4 起，配置可包含 `custom_rules`。每条规则由 `name`、`type`（`domain`/`ip`/`process-name`/`process-path`）、
`value` 和 `action`（`a`/`b`/`final`/`reject`）组成，最多 200 条。规则保留用户顺序，
位于 DNS/代理端点等防环路规则之后、接口直连与内置地域规则之前；规范化后相同匹配项的冲突动作会拒绝配置。
进程规则在用户规则组内优先于域名/IP；`process-name` 必须是无路径分隔符的 `.exe` 名称，`process-path`
必须是绝对 Windows 可执行文件路径。同名多路径在应用前失败关闭，子进程需使用自己的名称或路径显式配置。

远程规则集使用受限 JSON 文档：顶层仅允许 `version` 与 `rules`，每条远程规则使用与 `custom_rules`
相同的 `type`、`value` 和 `action`。来源必须是无 userinfo、query 和 fragment 的固定 HTTPS URL，并预先配置
SHA-256。更新仅在下载大小、哈希和结构全部验证后提交；失败时最后有效缓存保持不变。诊断包不包含规则正文。

1. sniff 元数据动作；
2. 回环与本机地址使用不绑定物理接口的 `local-direct`；
3. DNS 劫持、固定 DNS 上游和代理 endpoint 防环路规则；
4. 用户进程规则，保持进程规则之间的用户顺序；
5. 用户域名/IP 规则，保持域名/IP 规则之间的用户顺序；
6. InterfaceManager 提供的直连前缀按所属接口绑定；
7. 未被直连前缀覆盖的私有、链路本地、保留和测试地址拒绝；
8. IPv6 拒绝；
9. 国内域名和公网 CIDR 使用 `domestic-direct`；
10. `route.final` 使用当前模式唯一允许的最终出口。

不同出口的重叠直连前缀视为歧义并拒绝生成。相同输入先规范化、排序和去重，
始终产生相同 JSON。

## DNS 与 IPv6

国内 DNS detour 为 `domestic-direct`，全球 DNS detour 为
`foreign-direct`。`independent_cache` 始终启用，最终 DNS 为
`dns-global`，国内域名规则选择 `dns-domestic`。AAAA 查询规则首先拒绝，
TUN 同时配置 IPv6 地址并在路由层拒绝所有 IPv6，避免系统流量绕过。

UDP、DoT 和 DoH 可作为上游类型；DoT/DoH 要求固定 IP、端口和 TLS
`server_name`，DoH 路径固定为 `/dns-query`。

当前用户设置保存在 `%AppData%/WinRouter/dns-settings.json`，Schema 版本为 1。旧版本没有独立 DNS 文件时
迁移为阿里 UDP（国内线路）和 Google UDP（全球线路）；设置页也提供腾讯、Cloudflare 及加密 DNS 预设。
自定义值必须使用可用的固定 IP，端口范围为 1–65535，加密 DNS 必须提供不含路径的 TLS 域名，两个上游不能
完全相同。保存使用同目录临时文件原子替换；校验失败保留最后有效设置。后端在预览、验证和应用前统一注入持久化
设置，网络恢复复用最后成功应用的完整配置。诊断包只记录协议、线路出口及健康状态，不记录上游 IP。

## 节点收藏与测速

节点存储 Schema 2 在原有加密凭据和选择状态之外保存 `favorite`。Schema 1 启动时原子迁移；订阅更新按
协议、服务器和端口识别同一 endpoint 并保留收藏。测速结果不写入凭据存储，只保留在当前 UI 会话用于排序。

具体启停、回滚、迁移和限制见 [Phase 4 增强功能使用与回滚](./phase-4-enhancements.md)。

## 四层校验

1. 模型：版本、模式、接口、TUN、DNS、IPv6 和公网 CIDR 约束；
2. Schema：严格 JSON 解码、拒绝未知字段，并验证生成 JSON 必需结构；
3. 语义：规则顺序、tag 唯一性及引用、final、DNS 隔离和 IPv6 阻断；
4. 核心：校验 sing-box 版本和 SHA-256 后执行锁定版本 `sing-box check`。

任何一层失败均返回带 `model`、`schema`、`semantic` 或 `sing-box` 阶段的
错误，不输出可应用配置。
