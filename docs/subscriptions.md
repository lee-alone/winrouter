# Phase 3 订阅格式与安全边界

WinRouter Phase 3 接受 HTTPS 返回的版本 1 JSON 文档；仅当 URL 主机是字面量私网或回环 IP 时允许 HTTP，
例如 `http://192.168.1.50:3001/nodes.json`。HTTP 域名和公网 IP 仍被拒绝。响应上限为 1 MiB，
节点数量为 1 到 500；禁止重定向，拒绝未知字段、尾随 JSON 和未知协议。
订阅内容始终按数据解析，不执行其中的脚本、命令或路径。除版本 1 JSON 外，也接受整份 Base64 编码、
解码后每行一个标准 `ss://` URI 的 Shadowsocks 订阅；支持 SIP002 与旧式 Base64 URI，当前加密方法限
`aes-128/192/256-gcm`、`chacha20-ietf-poly1305` 和 `xchacha20-ietf-poly1305`。

```json
{
  "version": 1,
  "nodes": [
    {
      "name": "Example proxy",
      "type": "http",
      "server": "proxy.example.com",
      "port": 8080,
      "username": "optional-user",
      "password": "optional-password"
    }
  ]
}
```

订阅 URL（包括查询参数中的凭据）整体使用当前 Windows 用户的 DPAPI 加密落盘。
RPC 和日志仅返回订阅 ID、名称、主机名、节点数量、更新时间和固定错误描述。
更新先下载到内存并完成文档与全部节点校验，随后原子替换该订阅拥有的节点；
下载、格式、节点校验或文件替换失败时继续使用上一批有效节点。

代理域名使用配置中的固定全局 UDP DNS，通过网卡 B 的源 IPv4 查询。解析后的
IPv4 必须先通过 HTTP CONNECT 健康检查，成功后才原子写入节点状态并生成
endpoint `/32 -> foreign-direct` 规则。代理 outbound 还会显式绑定网卡 B，
两层约束共同防止服务器连接再次进入代理。
