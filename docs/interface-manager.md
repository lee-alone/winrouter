# InterfaceManager 设计与契约

## 职责

`internal/interfacemanager` 将 Windows 接口、路由、稳定标识、直连前缀和
TUN 前缀组合为供 Wails UI 与后续 PolicyEngine 使用的统一快照。底层
`internal/interfaces` 和 `internal/routes` 保持只读，不修改系统网络配置。

## 快照

快照包含：

- 全部物理与虚拟接口，以及过滤和排序后的物理候选接口；
- 完整路由表、直连前缀和跨接口重叠诊断；
- A/B 保存身份、当前匹配方法、可用状态和需重新选择原因；
- 确定性分配的 TUN 前缀及冲突诊断；
- 单调递增的进程内序列号。

候选接口仅包括 Ethernet 和 Wi-Fi。推荐优先级依次考虑接口为 `up`、存在
可用 IPv4、存在可用 IPv4 网关、存在 DNS，并以 IPv4 metric 和名称稳定排序。
选定 A/B 后还会按接口索引分别确认存在 IPv4 默认路由；任一出口缺失会产生阻断诊断。虚拟、
VPN、TUN、Hyper-V、WSL 和 Docker 接口仍保留在完整快照中，用于直连绕过
与冲突诊断，但不允许作为 A/B 候选。

## 标识与持久化

状态文件位于 `%AppData%\WinRouter\interfaces.json`，当前 Schema 版本为 3。版本 1/2 在内存中完成迁移和校验后才原子替换；迁移失败时保留原文件并阻止管理器启动及后续覆盖。状态同时保存 IPv6 策略，缺失时迁移为默认 `block`。
Interface GUID 是主持久标识；MAC、IPv4 前缀和友好名称是辅助证据。GUID
失效时仅返回匹配方法和候选结果，不自动改写保存的 GUID。歧义或证据不足
时状态为 `selection-required`，不得静默绑定。

状态通过同目录临时文件和 Windows `MoveFileEx` 原子替换。未知 Schema 或
损坏 JSON 会阻止 InterfaceManager 启动并保留原文件。

## 变化监听

管理器每秒重新读取 IP Helper 接口表和路由表，以语义指纹去抖。只有候选物理
接口或已选接口的增加/删除、状态、地址、网关、DNS、metric、相关默认路由、
阻断诊断或有效 TUN 前缀变化时，才发送 `interfaces:changed` Wails 事件。未选
虚拟接口、筛选驱动及其无关路由的变化不会刷新预检或触发恢复。接口地址、网关、
DNS 和路由在比较前按稳定顺序规范化，Windows API 仅改变返回顺序不会产生事件。
轮询不需要管理员权限，但短于一秒且在两次快照之间完全恢复的瞬态变化可能不产生
事件；启动预检仍会读取最终一致状态。

IPv6 策略为 `block` 时，IPv6 地址重叠只保留在拓扑信息中，不作为 IPv4 分流的阻断诊断；策略为 `split` 时，A/B 均必须具有可用 IPv6 地址和 IPv6 默认路由，且所有跨出口 IPv6 前缀重叠均为阻断诊断。

Wails 后端暴露：

- `GetInterfaceSnapshot()`：返回当前快照；
- `SelectInterfaces(interfaceAGUID, interfaceBGUID)`：验证两个不同物理接口，
  持久化身份并返回刷新后的快照。

## 安全边界

该组件只读取 Windows IP Helper 数据并写入当前用户配置目录，不需要提权。
创建 TUN、调整路由和启动核心仍由后续 helper 负责。
