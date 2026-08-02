# P0-TUN-01 最小 TUN 配置实验记录

## 实验状态

- 日期：2026-07-31
- 状态：当前单环境通过；第二环境列为 Phase 1 发布前兼容性补测
- 已完成：锁定核心、记录哈希与许可证、生成最小强类型配置、静态检查、提权启动、TUN 创建和地址确认、停止及残留资源检查
- Phase 0 已完成：正常信号停止路径验证和独立开发者三职责自审
- 发布前补测：第二套 Windows 环境复测

## 环境

- 操作系统：Microsoft Windows 10.0.26200 x64
- Go：项目构建环境见 `go version`
- sing-box：1.13.15
- Revision：`3708fa18766cda1f11b77f6ed9c7bd61688f17df`
- 可执行文件 SHA-256：`4DB8218DEA131668CCD5E0B32E773E916A37BE730E655B176E0D3A930276CBE7`
- 核心清单：`resources/core/manifest.json`
- 许可证：`resources/core/LICENSE`

## 配置意图

- TUN 地址从冲突检测后的私有 `/30` 地址池取得。
- `auto_route` 与 `strict_route` 启用。
- Phase 0 首个运行实验使用 `system` stack。
- 唯一 outbound 为 `direct`，`route.final` 明确指向该 outbound。
- `auto_detect_interface` 仅用于最小启动实验，双物理出口将在后续配置中显式绑定。

选择 `system` 作为首个实验对象是为了先验证 Windows 原生网络栈下的创建、路由和清理行为。`gvisor` 与 `mixed` 保留为兼容性对照；最终选择必须基于 Windows 10/11、Hyper-V、WSL、VPN 和 UDP 场景的运行证据。

## 已执行验证

以下命令均通过版本与 SHA-256 校验后调用锁定核心：

```powershell
.\bin\winrouter-probe.exe --check-tun --stack system
.\bin\winrouter-probe.exe --check-tun --stack gvisor
.\bin\winrouter-probe.exe --check-tun --stack mixed
```

三份配置均通过 sing-box 1.13.15 的 `check`，无标准错误输出。该结果只证明配置语法和静态语义有效，不证明 TUN 可成功启动。

## 提权运行结果

最终有效实验使用 `system` stack，证据文件为：

- `build/phase0/tun-system.stdout.json`
- `build/phase0/tun-system.stderr.log`

结果：

- 核心 PID：20552
- TUN 名称：`WinRouter-TUN`
- TUN GUID：`{9F0B9572-824A-32C4-E018-8DB69A267223}`
- TUN 地址：`172.19.0.1/30`
- 启动耗时：核心日志记录 0.27 秒
- 停止方式：实验器终止其启动的确切 PID
- 停止结果：进程退出，TUN 地址清理，接口相关路由为 0
- 核心日志量：5267 字符
- DNS：日志出现正常 `dns: exchanged` 结果，未再出现高速递归连接增长

实验迭代记录：

1. 普通权限启动被 Windows 拒绝，核心报告 `configure tun interface: Access is denied`；无残留资源。
2. 首次管理员启动成功，但缺少协议嗅探，TUN 网关 DNS 流量未命中 `hijack-dns`，约 3 秒产生 8.2 MB 日志。实验停止后资源清理正常。
3. 在 `hijack-dns` 前加入 `sniff` 动作后复测通过，DNS 循环消失。

## 后续验证

1. 增加正常控制信号停止路径，不只验证强制进程结束后的清理。
2. 对 `gvisor`、`mixed` 重复实验并记录 TCP、UDP 与兼容性差异。
3. 在 Phase 1 发布前于第二套 Windows 环境执行兼容性复测。
