import { computed, ref } from 'vue'
import {
  DeleteProxyNode,
  GetProxySelection,
  ListProxyNodes,
  SelectProxyNode,
  SetProxyNodeEgress,
  SetProxyNodeFavorite,
} from '../../wailsjs/go/app/App'
import type { nodes, ProxyProtocolMeta } from '../types'
import { ssMethods } from '../utils/proxyUri'
import { error, messageOf, notice } from './useFeedback'
import { applyProxySelection } from './useProxyChain'
import { proxyForm, resetProxyForm } from './useProxyNodeForm'
import { proxyTests } from './useProxyProbe'
import { subscriptionList } from './useSubscriptions'

export { ssMethods } from '../utils/proxyUri'
export * from '../utils/proxyUri'
export * from './useProxyNodeForm'

export const proxyProtocols: ProxyProtocolMeta[] = [
  { value: 'http', label: 'HTTP CONNECT', authKind: 'user-pass', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: true },
  { value: 'shadowsocks', label: 'Shadowsocks', authKind: 'ss', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: false },
  { value: 'vmess', label: 'VMess', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
  { value: 'vless', label: 'VLESS', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: true, allowNoSecret: false },
  { value: 'trojan', label: 'Trojan', authKind: 'trojan', supportsTLS: true, forceTLS: true, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
]

export const proxyNodes = ref<nodes.Node[]>([])
export const proxySort = ref<'favorite' | 'latency' | 'name'>('favorite')

export const selectedProxyNode = computed(() => proxyNodes.value.find(node => node.selected))

export const sortedProxyNodes = computed(() =>
  [...proxyNodes.value].sort((first, second) => {
    if (proxySort.value === 'favorite' && first.favorite !== second.favorite) return first.favorite ? -1 : 1
    if (proxySort.value === 'latency') {
      const aTest = proxyTests.value[first.id]
      const bTest = proxyTests.value[second.id]
      const a = aTest?.available ? aTest.total_ms || aTest.latency_ms : Number.MAX_SAFE_INTEGER
      const b = bTest?.available ? bTest.total_ms || bTest.latency_ms : Number.MAX_SAFE_INTEGER
      if (a !== b) return a - b
    }
    return first.name.localeCompare(second.name, 'zh-CN')
  })
)

export const proxyRiskMessages = computed(() => {
  const result: string[] = []
  const node = selectedProxyNode.value
  if (!node) result.push('未选择代理节点，代理模式无法启动。')
  else {
    if (node.server.includes('.') && !/^\d+\.\d+\.\d+\.\d+$/.test(node.server) && !node.resolved_ip)
      result.push('代理域名尚未通过出口 B DNS 解析和健康检查。')
    const tested = proxyTests.value[node.id]
    if (tested && !tested.available) result.push('所选代理最近一次可用性测试失败，启动将严格失败且不会直连回退。')
  }
  if (subscriptionList.value.some(item => item.last_error)) result.push('最近一次订阅更新失败，应用已保留上一批有效节点。')
  return result
})

export function formatProxySummary(node: nodes.Node): string[] {
  const parts: string[] = []
  if (node.type === 'http') {
    parts.push('HTTP CONNECT')
    if (node.authentication?.username || node.username) {
      parts.push(`用户: ${node.authentication?.username || node.username}`)
    }
  } else if (node.type === 'shadowsocks') {
    parts.push(`SS · ${node.authentication?.method || node.username || 'aes-256-gcm'}`)
  } else if (node.type === 'vmess') {
    parts.push('VMess')
  } else if (node.type === 'vless') {
    parts.push('VLESS')
    if (node.authentication?.flow) {
      parts.push(node.authentication.flow)
    }
  } else if (node.type === 'trojan') {
    parts.push('Trojan')
  } else {
    parts.push(node.type.toUpperCase())
  }

  if (node.type === 'trojan' || node.tls?.enabled) {
    let tlsText = 'TLS'
    const sub: string[] = []
    if (node.tls?.server_name) sub.push(`SNI: ${node.tls.server_name}`)
    if (node.tls?.alpn?.length) sub.push(`ALPN: ${node.tls.alpn.join(',')}`)
    if (node.tls?.insecure) sub.push('Insecure')
    if (sub.length) tlsText += ` (${sub.join(' · ')})`
    parts.push(tlsText)
  }

  if (node.transport?.type === 'ws') {
    let wsText = 'WS'
    const wsSub: string[] = []
    if (node.transport.path) wsSub.push(node.transport.path)
    if (node.transport.host) wsSub.push(`host: ${node.transport.host}`)
    if (wsSub.length) wsText += ` (${wsSub.join(' · ')})`
    parts.push(wsText)
  } else if (node.type === 'vmess' || node.type === 'vless' || node.type === 'trojan') {
    parts.push('TCP')
  }

  return parts
}

export async function copyProxyNodeInfo(node: nodes.Node): Promise<void> {
  try {
    let text = `${node.name}\n`
    text += `类型: ${node.type === 'http' ? 'HTTP CONNECT' : node.type.toUpperCase()}\n`
    text += `服务器: ${node.server}:${node.port}\n`
    text += `出口: 接口 ${node.egress.toUpperCase()}\n`
    if (node.authentication?.username) text += `用户名: ${node.authentication.username}\n`
    if (node.authentication?.method) text += `加密方法: ${node.authentication.method}\n`
    if (node.authentication?.flow) text += `Flow: ${node.authentication.flow}\n`
    if (node.tls?.enabled || node.type === 'trojan') {
      text += `TLS: 启用`
      if (node.tls?.server_name) text += ` (SNI: ${node.tls.server_name})`
      if (node.tls?.alpn?.length) text += ` (ALPN: ${node.tls.alpn.join(', ')})`
      if (node.tls?.insecure) text += ` (Insecure)`
      text += `\n`
    }
    if (node.transport?.type === 'ws') {
      text += `传输: WebSocket`
      if (node.transport.path) text += ` (Path: ${node.transport.path})`
      if (node.transport.host) text += ` (Host: ${node.transport.host})`
      text += `\n`
    }
    await navigator.clipboard.writeText(text)
    notice.value = `已复制节点“${node.name}”配置摘要到剪贴板。`
  } catch (reason) {
    error.value = `复制失败：${messageOf(reason)}`
  }
}

export async function refreshProxyState(): Promise<void> {
  try {
    const [nodesList, sel] = await Promise.all([ListProxyNodes(), GetProxySelection()])
    const list = Array.isArray(nodesList) ? nodesList : []
    proxyNodes.value = list
    applyProxySelection(sel, list)
  } catch (reason) {
    console.error('Failed to refresh proxy state:', reason)
  }
}

export async function selectProxyNode(id: string, isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止切换代理节点；请先停止核心。'
    return
  }
  try {
    await SelectProxyNode(id)
    await refreshProxyState()
    notice.value = '默认代理节点已更新。'
  } catch (reason) {
    error.value = `无法选择代理节点：${messageOf(reason)}`
  }
}

export async function deleteProxyNode(node: nodes.Node, isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止删除代理节点；请先停止核心。'
    return
  }
  if (!window.confirm(`删除代理节点“${node.name}”？此操作会同时删除其加密凭据。`)) return
  try {
    await DeleteProxyNode(node.id)
    await refreshProxyState()
    notice.value = '代理节点及其加密凭据已删除。'
    if (proxyForm.value.id === node.id) resetProxyForm()
  } catch (reason) {
    error.value = `无法删除代理节点：${messageOf(reason)}`
  }
}

export async function toggleProxyFavorite(node: nodes.Node): Promise<void> {
  try {
    await SetProxyNodeFavorite(node.id, !node.favorite)
    await refreshProxyState()
  } catch (reason) {
    error.value = `无法更新收藏：${messageOf(reason)}`
  }
}

export async function toggleProxyNodeEgress(node: nodes.Node, isRunning = false): Promise<void> {
  if (isRunning) return
  error.value = ''
  const nextEgress = node.egress === 'a' ? 'b' : 'a'
  try {
    await SetProxyNodeEgress(node.id, nextEgress)
    await refreshProxyState()
    notice.value = `节点“${node.name}”出口已切换为出口 ${nextEgress.toUpperCase()}`
  } catch (reason) {
    error.value = `无法修改节点出口：${messageOf(reason)}`
  }
}
