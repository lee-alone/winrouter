# 提权 Helper 与核心控制

## 形态决策

Phase 1 采用按需提权进程，不安装常驻 Windows Service。依据如下：

- Phase 0 已验证 Job Object 在父进程强杀时终止核心；
- 按需进程减少常驻高权限代码和服务安装/升级面；
- UI 始终以普通权限运行，仅在用户应用配置时通过 `runas` 触发 UAC；
- UI 每 5 秒发送认证 heartbeat，20 秒无认证活动时 helper 停止核心并退出。

## RPC 边界

Named Pipe 名称由当前用户 SID 确定。DACL 只允许当前用户读写，并允许
Administrators 和 SYSTEM 完全访问。每次 helper 会话使用 256 位随机令牌，
服务端只保存 SHA-256，并以常量时间比较验证。

协议使用 4 字节长度帧，单条消息上限 4 MiB。唯一允许的方法为：

- `validate`：验证 Phase 1 `MVPConfig`；
- `apply`：生成并应用 Phase 1 `MVPConfig`；
- `stop`：停止核心；
- `status`：查询结构化状态；
- `heartbeat`：维持当前 UI 会话。

专用崩溃验收构建还可启用无参数的 `fault-terminate-core`。普通构建在编译期关闭
该能力，即使传入测试命令行开关也会拒绝启动；该方法不属于用户发布版 RPC 面。

RPC 不接受可执行文件、命令行或文件路径。helper 不接受任意 sing-box JSON，
而是在提权边界内重新严格解码 `MVPConfig`、拒绝未知字段，再调用确定性
ConfigGenerator。核心和状态目录均由 helper 可执行文件位置及 ProgramData
固定推导。

## 配置事务

配置按以下顺序应用：模型/Schema/语义校验、锁定核心版本和 SHA-256 校验、
`sing-box check`、写入固定 `candidate.json`、停止旧核心、启动候选、等待 TUN
接口与地址健康、原子提交 `active.json` 和 `last-good.json`。候选启动失败时，
控制器重新启动内存或磁盘中的最后有效配置。

状态文件使用同目录临时文件、仅当前主体读写的文件模式及 Windows
`MoveFileEx(REPLACE_EXISTING|WRITE_THROUGH)`。启动时只删除本产品固定名称的
`candidate.json` 和 `rollback.json`，不会扫描或删除任意路径。

## 生命周期

核心在 `KILL_ON_JOB_CLOSE` Job Object 中运行。正常停止先发送 CTRL_BREAK，
5 秒无响应才强制结束。异常退出写入 `abnormal.json`，并以 1、2、4 秒退避，
最多重启 3 次；达到上限后保持失败状态，不无限循环。

生产健康检查要求核心进程存活，且配置声明的 `WinRouter-TUN` 地址出现在
状态为 `up` 的接口上。核心输出使用 256 KiB 有界缓冲，避免日志导致 helper
内存无限增长。

## 发布约束

开发构建未签名，位于用户可写工作区。正式安装必须将 UI、helper 和核心放入
受管理员 ACL 保护的安装目录并完成代码签名，防止 helper 二进制替换。该要求
属于 Phase 1 发布检查，不影响当前源码和受控开发环境验证结论。
