import { computed, ref } from 'vue'
import {
  AddProxyNode,
  AddSubscription,
  DeleteProxyNode,
  DeleteSubscription,
  GetProxySelection,
  ListProxyNodes,
  ListSubscriptions,
  RefreshSubscription,
  SelectProxyChain,
  SelectProxyNode,
  SetProxyMode,
  SetProxyNodeEgress,
  SetProxyNodeFavorite,
  SpeedTestProxyNodes,
  TestProxyChain,
  TestProxyNode,
  UpdateProxyNode,
  UpdateSubscription,
} from '../../wailsjs/go/app/App'
import type { nodes, subscriptions } from '../../wailsjs/go/models'
import type { ProxyProtocolMeta } from '../types'
import { getBootstrapDNSServer } from './useDNSManager'
import { busy, error, messageOf, notice } from './useFeedback'

export const proxyProtocols: ProxyProtocolMeta[] = [
  { value: 'http', label: 'HTTP CONNECT', authKind: 'user-pass', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: true },
  { value: 'shadowsocks', label: 'Shadowsocks', authKind: 'ss', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: false },
  { value: 'vmess', label: 'VMess', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
  { value: 'vless', label: 'VLESS', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: true, allowNoSecret: false },
  { value: 'trojan', label: 'Trojan', authKind: 'trojan', supportsTLS: true, forceTLS: true, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
]

export const ssMethods = [
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
]

export const proxyNodes = ref<nodes.Node[]>([])
export const proxyFormOpen = ref(false)
export const proxyForm = ref({
  id: '',
  original_type: 'http' as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan',
  original_has_secret: false,
  name: '',
  type: 'http' as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan',
  server: '',
  port: 8080,
  egress: 'b' as 'a' | 'b',
  username: '',
  password: '',
  method: 'aes-256-gcm',
  uuid: '',
  flow: '',
  tls_enabled: false,
  tls_server_name: '',
  tls_insecure: false,
  tls_alpn: '',
  transport_type: 'tcp' as 'tcp' | 'ws',
  transport_path: '',
  transport_host: '',
  has_saved_secret: false,
  replace_secret: false,
  clear_secret: false,
})

export const proxyFieldErrors = ref<Record<string, string>>({})
export const proxyImportOpen = ref(false)
export const proxyImportURI = ref('')
export const proxyImportError = ref('')
export const proxyTests = ref<Record<string, nodes.TestResult>>({})
export const testingProxyID = ref('')
export const testingAllProxies = ref(false)
export const proxySort = ref<'favorite' | 'latency' | 'name'>('favorite')
export const proxySelection = ref<nodes.ProxySelection>({ mode: 'single', selected_id: '', selected_chain: [] })
export const chainTestResult = ref<nodes.TestResult | null>(null)
export const testingChain = ref(false)
export const subscriptionList = ref<subscriptions.Subscription[]>([])
export const subscriptionFormOpen = ref(false)
export const subscriptionForm = ref<{ id: string; name: string; url: string; egress: 'a' | 'b' }>({ id: '', name: '', url: '', egress: 'b' })
export const refreshingSubscriptionID = ref('')

export const selectedProxyNode = computed(() => proxyNodes.value.find(node => node.selected))
export const isChainMode = computed(() => proxySelection.value?.mode === 'chain' && (proxyNodes.value?.length || 0) > 0)
export const chainNodes = computed(() => {
  const list = proxyNodes.value || []
  const map = new Map(list.map(n => [n.id, n]))
  const chainIds = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  return chainIds.map(id => map.get(id)).filter((n): n is nodes.Node => Boolean(n))
})
export const isChainReady = computed(() => isChainMode.value && chainNodes.value.length >= 2)
export const chainSummaryText = computed(() => {
  if (!chainNodes.value || !chainNodes.value.length) return '未配置链路'
  return chainNodes.value.map(n => n?.name || '未知节点').join(' ➔ ')
})

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

export function formatErrorCategory(cat?: string) {
  switch (cat) {
    case 'dns_failed': return 'DNS 解析失败'
    case 'egress_unavailable': return '出口网卡不可用'
    case 'tcp_failed': return 'TCP 连接失败'
    case 'tls_failed': return 'TLS 握手失败'
    case 'authentication_failed': return '凭据认证失败'
    case 'protocol_failed': return '协议通信失败'
    case 'target_failed': return '测试目标异常'
    case 'timeout': return '请求超时'
    case 'core_failed': return '临时核心异常'
    default: return '测试失败'
  }
}

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

export function safeBase64Decode(raw: string): string {
  let str = raw.trim().replace(/-/g, '+').replace(/_/g, '/')
  while (str.length % 4 !== 0) {
    str += '='
  }
  const binary = atob(str)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return new TextDecoder('utf-8').decode(bytes)
}

export function parseProxyURI(uri: string) {
  const trimmed = uri.trim()
  if (!trimmed) throw new Error('链接内容为空')

  if (trimmed.startsWith('ss://')) {
    let body = trimmed.slice(5)
    let tag = ''
    const hashIdx = body.indexOf('#')
    if (hashIdx !== -1) {
      tag = decodeURIComponent(body.slice(hashIdx + 1))
      body = body.slice(0, hashIdx)
    }
    let method = 'aes-256-gcm'
    let password = ''
    let server = ''
    let port = 8388

    if (body.includes('@')) {
      const atIdx = body.indexOf('@')
      const userInfoEncoded = body.slice(0, atIdx)
      const serverPort = body.slice(atIdx + 1)
      let userInfo = ''
      try {
        userInfo = safeBase64Decode(userInfoEncoded)
      } catch {
        userInfo = userInfoEncoded
      }
      const colonIdx = userInfo.indexOf(':')
      if (colonIdx !== -1) {
        method = userInfo.slice(0, colonIdx)
        password = userInfo.slice(colonIdx + 1)
      }
      const lastColon = serverPort.lastIndexOf(':')
      if (lastColon !== -1) {
        server = serverPort.slice(0, lastColon)
        port = parseInt(serverPort.slice(lastColon + 1), 10) || 8388
      } else {
        server = serverPort
      }
    } else {
      let decoded = ''
      try {
        decoded = safeBase64Decode(body)
      } catch {
        throw new Error('无效的 Shadowsocks base64 编码')
      }
      const atIdx = decoded.indexOf('@')
      if (atIdx !== -1) {
        const userInfo = decoded.slice(0, atIdx)
        const serverPort = decoded.slice(atIdx + 1)
        const colonIdx = userInfo.indexOf(':')
        if (colonIdx !== -1) {
          method = userInfo.slice(0, colonIdx)
          password = userInfo.slice(colonIdx + 1)
        }
        const lastColon = serverPort.lastIndexOf(':')
        if (lastColon !== -1) {
          server = serverPort.slice(0, lastColon)
          port = parseInt(serverPort.slice(lastColon + 1), 10) || 8388
        } else {
          server = serverPort
        }
      }
    }

    const methodLower = method.toLowerCase().trim()
    if (!ssMethods.includes(methodLower)) {
      throw new Error(`不支持的 Shadowsocks 加密方法 "${method}"，后端当前仅支持: ${ssMethods.join(', ')}`)
    }

    return {
      name: tag || `${server}:${port}`,
      type: 'shadowsocks' as const,
      server,
      port,
      method: methodLower,
      password,
    }
  }

  if (trimmed.startsWith('vmess://')) {
    const b64 = trimmed.slice(8)
    let jsonStr = ''
    try {
      jsonStr = safeBase64Decode(b64)
    } catch {
      throw new Error('无效的 VMess base64 编码')
    }
    const data = JSON.parse(jsonStr)
    const vmessAlpn = (data.alpn || '').trim()
    return {
      name: data.ps || `${data.add}:${data.port}`,
      type: 'vmess' as const,
      server: data.add || '',
      port: Number(data.port) || 443,
      uuid: data.id || '',
      tls_enabled: data.tls === 'tls' || data.tls === '1' || data.tls === 'true',
      tls_server_name: data.sni || data.host || data.add || '',
      transport_type: (data.net === 'ws' ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: data.path || '',
      transport_host: data.host || '',
      tls_alpn: vmessAlpn.toLowerCase() === 'default' ? '' : vmessAlpn,
    }
  }

  if (trimmed.startsWith('vless://')) {
    const url = new URL(trimmed)
    const uuid = url.username
    const server = url.hostname
    const port = parseInt(url.port, 10) || 443
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    const params = url.searchParams
    const security = (params.get('security') || '').toLowerCase()
    if (security === 'reality') {
      throw new Error('暂不支持 VLESS Reality 协议链接（需要服务端与客户端 Reality 公钥及 Short ID 支持）')
    }
    const isTLS = security === 'tls'
    const isWS = params.get('type') === 'ws'
    const alpn = (params.get('alpn') || '').trim()
    return {
      name: tag || `${server}:${port}`,
      type: 'vless' as const,
      server,
      port,
      uuid,
      flow: params.get('flow') || '',
      tls_enabled: isTLS,
      tls_server_name: params.get('sni') || params.get('peer') || server || '',
      tls_insecure: params.get('allowInsecure') === '1' || params.get('insecure') === '1',
      tls_alpn: alpn.toLowerCase() === 'default' ? '' : alpn,
      transport_type: (isWS ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: params.get('path') || '',
      transport_host: params.get('host') || '',
    }
  }

  if (trimmed.startsWith('trojan://')) {
    const url = new URL(trimmed)
    const password = url.username
    const server = url.hostname
    const port = parseInt(url.port, 10) || 443
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    const params = url.searchParams
    const isWS = params.get('type') === 'ws'
    const trojanAlpn = (params.get('alpn') || '').trim()
    return {
      name: tag || `${server}:${port}`,
      type: 'trojan' as const,
      server,
      port,
      password,
      tls_enabled: true,
      tls_server_name: params.get('sni') || params.get('peer') || server || '',
      tls_insecure: params.get('allowInsecure') === '1' || params.get('insecure') === '1',
      tls_alpn: trojanAlpn.toLowerCase() === 'default' ? '' : trojanAlpn,
      transport_type: (isWS ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: params.get('path') || '',
      transport_host: params.get('host') || '',
    }
  }

  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    const url = new URL(trimmed)
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    return {
      name: tag || `${url.hostname}:${url.port || 8080}`,
      type: 'http' as const,
      server: url.hostname,
      port: parseInt(url.port, 10) || 8080,
      username: url.username ? decodeURIComponent(url.username) : '',
      password: url.password ? decodeURIComponent(url.password) : '',
    }
  }

  throw new Error('不支持的链接协议，支持 ss://, vmess://, vless://, trojan://, http://')
}

export function openImportBox() {
  proxyImportURI.value = ''
  proxyImportError.value = ''
  proxyImportOpen.value = true
}

export function closeImportBox() {
  proxyImportOpen.value = false
  proxyImportURI.value = ''
  proxyImportError.value = ''
}

export function openNewProxyForm() {
  resetProxyForm()
  proxyFormOpen.value = true
}

export function selectProtocol(type: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan') {
  if (proxyForm.value.type === type) return
  proxyForm.value.type = type
  if (type === 'trojan') {
    proxyForm.value.tls_enabled = true
  }

  if (proxyForm.value.id) {
    if (type === proxyForm.value.original_type) {
      proxyForm.value.has_saved_secret = proxyForm.value.original_has_secret
      proxyForm.value.replace_secret = !proxyForm.value.original_has_secret
      proxyForm.value.clear_secret = false
    } else {
      proxyForm.value.has_saved_secret = false
      proxyForm.value.replace_secret = true
      proxyForm.value.clear_secret = false
      if (type === 'vmess' || type === 'vless') {
        proxyForm.value.password = ''
      } else if (type === 'shadowsocks' || type === 'trojan' || type === 'http') {
        proxyForm.value.uuid = ''
        proxyForm.value.flow = ''
      }
    }
  }
  proxyFieldErrors.value = {}
}

export function resetProxyForm() {
  proxyForm.value = {
    id: '',
    original_type: 'http',
    original_has_secret: false,
    name: '',
    type: 'http',
    server: '',
    port: 8080,
    egress: 'b',
    username: '',
    password: '',
    method: 'aes-256-gcm',
    uuid: '',
    flow: '',
    tls_enabled: false,
    tls_server_name: '',
    tls_insecure: false,
    tls_alpn: '',
    transport_type: 'tcp',
    transport_path: '',
    transport_host: '',
    has_saved_secret: false,
    replace_secret: false,
    clear_secret: false,
  }
  proxyFieldErrors.value = {}
  proxyFormOpen.value = false
}

export function editProxyNode(node: nodes.Node) {
  const hasSecret = Boolean(node.has_secret || node.has_password)
  const nodeType = ((node.type as any) || 'http') as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  proxyForm.value = {
    id: node.id,
    original_type: nodeType,
    original_has_secret: hasSecret,
    name: node.name,
    type: nodeType,
    server: node.server,
    port: node.port,
    egress: (node.egress as any) || 'b',
    username: node.authentication?.username || node.username || '',
    password: '',
    method: node.authentication?.method || (node.type === 'shadowsocks' ? node.username : 'aes-256-gcm') || 'aes-256-gcm',
    uuid: '',
    flow: node.authentication?.flow || '',
    tls_enabled: node.tls?.enabled || node.type === 'trojan',
    tls_server_name: node.tls?.server_name || '',
    tls_insecure: node.tls?.insecure || false,
    tls_alpn: node.tls?.alpn ? node.tls.alpn.join(', ') : '',
    transport_type: node.transport?.type === 'ws' ? 'ws' : 'tcp',
    transport_path: node.transport?.path || '',
    transport_host: node.transport?.host || '',
    has_saved_secret: hasSecret,
    replace_secret: !hasSecret,
    clear_secret: false,
  }
  proxyFieldErrors.value = {}
  proxyFormOpen.value = true
}

export function validateProxyForm(): boolean {
  proxyFieldErrors.value = {}
  const f = proxyForm.value
  const protoMeta = proxyProtocols.find(p => p.value === f.type)
  let isValid = true

  if (!f.name || !f.name.trim()) {
    proxyFieldErrors.value.name = '请输入节点名称'
    isValid = false
  } else if (f.name.length > 80) {
    proxyFieldErrors.value.name = '节点名称最多 80 个字符'
    isValid = false
  }

  if (!f.server || !f.server.trim()) {
    proxyFieldErrors.value.server = '请输入服务器地址（IPv4 或域名）'
    isValid = false
  } else if (f.server.includes(' ')) {
    proxyFieldErrors.value.server = '服务器地址不能包含空格'
    isValid = false
  }

  if (!f.port || f.port < 1 || f.port > 65535) {
    proxyFieldErrors.value.port = '端口必须在 1-65535 之间'
    isValid = false
  }

  const isProtocolChanged = Boolean(f.id && f.type !== f.original_type)
  const hasSavedSecretForCurrentProtocol = f.has_saved_secret && !isProtocolChanged
  const needsSecret = !f.id || f.replace_secret || !hasSavedSecretForCurrentProtocol
  if (needsSecret && !f.clear_secret) {
    if (f.type === 'shadowsocks' && !f.password) {
      proxyFieldErrors.value.password = 'Shadowsocks 密码为必填项'
      isValid = false
    } else if (f.type === 'trojan' && !f.password) {
      proxyFieldErrors.value.password = 'Trojan 密码为必填项'
      isValid = false
    } else if (f.type === 'vmess' || f.type === 'vless') {
      if (!f.uuid || !f.uuid.trim()) {
        proxyFieldErrors.value.uuid = `${f.type.toUpperCase()} UUID 为必填项`
        isValid = false
      } else {
        const uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/
        if (!uuidRegex.test(f.uuid.trim())) {
          proxyFieldErrors.value.uuid = 'UUID 格式无效，必须为 36 位标准 UUID（如 8-4-4-4-12）'
          isValid = false
        }
      }
    }
  }

  const isTLSEnabled = Boolean(protoMeta?.supportsTLS && (protoMeta.forceTLS || f.tls_enabled))
  if (isTLSEnabled) {
    if (f.tls_server_name && (f.tls_server_name.includes(' ') || f.tls_server_name.length > 253)) {
      proxyFieldErrors.value.tls_server_name = 'TLS Server Name 不能包含空格且长度不超过 253'
      isValid = false
    }
    if (f.tls_alpn) {
      const tokens = f.tls_alpn.split(/[,;\s]+/).map(s => s.trim()).filter(Boolean)
      for (const tok of tokens) {
        if (tok.length > 32) {
          proxyFieldErrors.value.tls_alpn = `ALPN 单项 "${tok}" 长度超过 32 字符`
          isValid = false
          break
        }
      }
    }
  }

  const isWSEnabled = Boolean(protoMeta?.supportsTransport && f.transport_type === 'ws')
  if (isWSEnabled) {
    if (f.transport_path) {
      if (!f.transport_path.startsWith('/')) {
        proxyFieldErrors.value.transport_path = 'WebSocket Path 必须以 "/" 开头（例如 /ws）'
        isValid = false
      } else if (f.transport_path.length > 2048) {
        proxyFieldErrors.value.transport_path = 'WebSocket Path 长度不能超过 2048 字符'
        isValid = false
      }
    }
    if (f.transport_host && (f.transport_host.includes(' ') || f.transport_host.length > 253)) {
      proxyFieldErrors.value.transport_host = 'WebSocket Host 不能包含空格且长度不超过 253'
      isValid = false
    }
  }

  return isValid
}

export async function saveProxyNode(isRunning = false) {
  error.value = ''
  notice.value = ''
  if (!validateProxyForm()) {
    error.value = '请更正表单中的输入错误。'
    return
  }
  if (isRunning) {
    error.value = '核心运行中，禁止修改或添加节点；请先停止核心。'
    return
  }
  busy.value = true
  try {
    const f = proxyForm.value
    const protoMeta = proxyProtocols.find(p => p.value === f.type)
    const alpnTokens = f.tls_alpn ? f.tls_alpn.split(/[,;\s]+/).map(s => s.trim()).filter(Boolean) : undefined
    const isTLSEnabled = Boolean(protoMeta?.supportsTLS && (protoMeta.forceTLS || f.tls_enabled))
    const isWSEnabled = Boolean(protoMeta?.supportsTransport && f.transport_type === 'ws')

    const authInput: nodes.AuthenticationInput = {
      username: f.type === 'http' ? f.username?.trim() || undefined : undefined,
      password: f.type === 'http' || f.type === 'shadowsocks' || f.type === 'trojan' ? f.password || undefined : undefined,
      method: f.type === 'shadowsocks' ? f.method || undefined : undefined,
      uuid: f.type === 'vmess' || f.type === 'vless' ? f.uuid?.trim() || undefined : undefined,
      flow: protoMeta?.supportsFlow && f.type === 'vless' ? f.flow || undefined : undefined,
    }

    const tlsInput: nodes.TLSInput | undefined = isTLSEnabled
      ? {
          enabled: true,
          server_name: f.tls_server_name?.trim() || undefined,
          insecure: f.tls_insecure || false,
          alpn: alpnTokens && alpnTokens.length ? alpnTokens : undefined,
        }
      : undefined

    const transportInput: nodes.TransportInput | undefined = isWSEnabled
      ? {
          type: 'ws',
          path: f.transport_path?.trim() || undefined,
          host: f.transport_host?.trim() || undefined,
        }
      : undefined

    const hasNewSecret = Boolean(authInput.password || authInput.uuid)
    const isProtocolChanged = Boolean(f.id && f.type !== f.original_type)
    const shouldClearSecret = f.clear_secret || (isProtocolChanged && !hasNewSecret)

    const input = {
      id: f.id || undefined,
      name: f.name.trim(),
      type: f.type,
      server: f.server.trim(),
      port: f.port,
      egress: f.egress,
      clear_secret: shouldClearSecret,
      authentication: authInput,
      tls: tlsInput,
      transport: transportInput,
    } as unknown as nodes.Input

    if (input.id) await UpdateProxyNode(input)
    else await AddProxyNode(input)
    await refreshProxyState()
    if (input.id) {
      if (shouldClearSecret) notice.value = '代理节点已更新，已清除旧凭据。'
      else if (f.replace_secret || hasNewSecret) notice.value = '代理节点已更新，新凭据已安全保存。'
      else notice.value = '代理节点已更新，已保留原有加密凭据。'
    } else {
      notice.value = '代理节点已添加并安全保存。'
    }
    resetProxyForm()
  } catch (reason) {
    error.value = `无法保存代理节点：${messageOf(reason)}`
  } finally {
    busy.value = false
  }
}

export function applyImportURI() {
  proxyImportError.value = ''
  try {
    const parsed = parseProxyURI(proxyImportURI.value)
    resetProxyForm()
    proxyForm.value = {
      id: '',
      original_type: parsed.type,
      original_has_secret: false,
      name: parsed.name || '',
      type: parsed.type,
      server: parsed.server,
      port: parsed.port,
      egress: 'b',
      username: (parsed as any).username || '',
      password: (parsed as any).password || '',
      method: (parsed as any).method || 'aes-256-gcm',
      uuid: (parsed as any).uuid || '',
      flow: (parsed as any).flow || '',
      tls_enabled: (parsed as any).tls_enabled || parsed.type === 'trojan',
      tls_server_name: (parsed as any).tls_server_name || '',
      tls_insecure: (parsed as any).tls_insecure || false,
      tls_alpn: (parsed as any).tls_alpn || '',
      transport_type: (parsed as any).transport_type || 'tcp',
      transport_path: (parsed as any).transport_path || '',
      transport_host: (parsed as any).transport_host || '',
      has_saved_secret: false,
      replace_secret: true,
      clear_secret: false,
    }
    proxyFormOpen.value = true
    closeImportBox()
    notice.value = `已从链接导入节点“${proxyForm.value.name}”，请核对并保存。`
  } catch (reason) {
    proxyImportError.value = `导入解析失败：${messageOf(reason)}`
  }
}

export async function copyProxyNodeInfo(node: nodes.Node) {
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

export async function refreshProxyState() {
  try {
    const [nodesList, sel] = await Promise.all([ListProxyNodes(), GetProxySelection()])
    const list = Array.isArray(nodesList) ? nodesList : []
    proxyNodes.value = list
    const selChain = Array.isArray(sel?.selected_chain) ? sel.selected_chain : []
    const validChain = selChain.filter(id => list.some(n => n.id === id))
    const rawMode = sel?.mode || 'single'
    const mode = rawMode === 'chain' && list.length > 0 ? 'chain' : 'single'
    proxySelection.value = {
      mode,
      selected_id: sel?.selected_id || '',
      selected_chain: validChain,
    }
  } catch (reason) {
    console.error('Failed to refresh proxy state:', reason)
  }
}

export async function selectProxyNode(id: string, isRunning = false) {
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

export async function switchProxyMode(mode: 'single' | 'chain', isRunning = false) {
  if (isRunning) {
    error.value = '核心运行中，禁止切换代理模式；请先停止核心。'
    return
  }
  if (mode === 'chain' && (!proxyNodes.value || proxyNodes.value.length === 0)) {
    notice.value = '尚未添加代理节点，请先添加或导入节点后再启用链式套接模式。'
    return
  }
  error.value = ''
  try {
    await SetProxyMode(mode)
    await refreshProxyState()
    notice.value = mode === 'chain' ? '已切换至链式套接模式。' : '已切换至单节点直连模式。'
  } catch (reason) {
    error.value = `切换代理模式失败：${messageOf(reason)}`
  }
}

export function isNodeInChain(id: string): boolean {
  return Array.isArray(proxySelection.value?.selected_chain) && proxySelection.value.selected_chain.includes(id)
}

export function chainNodeHopIndex(id: string): number {
  return Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain.indexOf(id) : -1
}

export async function addToChain(node: nodes.Node, isRunning = false) {
  if (isRunning) {
    error.value = '核心运行中，禁止修改代理链路；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (curChain.includes(node.id)) return
  const nextChain = [...curChain, node.id]
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
    notice.value = `已将“${node.name}”添加到代理链路末端。`
  } catch (reason) {
    error.value = `添加链路节点失败：${messageOf(reason)}`
  }
}

export async function removeFromChain(index: number, isRunning = false) {
  if (isRunning) {
    error.value = '核心运行中，禁止修改代理链路；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (index < 0 || index >= curChain.length) return
  const nextChain = [...curChain]
  const removedId = nextChain.splice(index, 1)[0]
  const nodeName = proxyNodes.value.find(n => n.id === removedId)?.name || '节点'
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
    notice.value = `已从链路中移除“${nodeName}”。`
  } catch (reason) {
    error.value = `移除链路节点失败：${messageOf(reason)}`
  }
}

export async function moveChainHop(index: number, direction: 'up' | 'down', isRunning = false) {
  if (isRunning) {
    error.value = '核心运行中，禁止调整链路顺序；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  const targetIndex = direction === 'up' ? index - 1 : index + 1
  if (targetIndex < 0 || targetIndex >= curChain.length || index < 0 || index >= curChain.length) return
  const nextChain = [...curChain]
  const [moved] = nextChain.splice(index, 1)
  nextChain.splice(targetIndex, 0, moved)
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
  } catch (reason) {
    error.value = `调整链路顺序失败：${messageOf(reason)}`
  }
}

export async function clearChain(isRunning = false) {
  if (isRunning) {
    error.value = '核心运行中，禁止清空代理链路；请先停止核心。'
    return
  }
  try {
    await SelectProxyChain([])
    await refreshProxyState()
    chainTestResult.value = null
    notice.value = '已清空代理链路。'
  } catch (reason) {
    error.value = `清空代理链路失败：${messageOf(reason)}`
  }
}

export async function testChain() {
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (curChain.length < 2) {
    error.value = '套接链至少需要 2 个节点才能进行整链测试。'
    return
  }
  testingChain.value = true
  error.value = ''
  try {
    const result = await TestProxyChain(curChain, getBootstrapDNSServer())
    chainTestResult.value = result
    if (result.available) {
      notice.value = `整链测试成功！延迟 ${result.latency_ms} ms。`
    } else {
      error.value = `整链测试不可用：${formatErrorCategory(result.error_category)}${result.error ? ` (${result.error})` : ''}`
    }
  } catch (reason) {
    error.value = `整链测试失败：${messageOf(reason)}`
  } finally {
    testingChain.value = false
  }
}

export async function deleteProxyNode(node: nodes.Node, isRunning = false) {
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

export async function testProxy(node: nodes.Node) {
  testingProxyID.value = node.id
  error.value = ''
  try {
    const result = await TestProxyNode(node.id, getBootstrapDNSServer())
    proxyTests.value = { ...proxyTests.value, [node.id]: result }
  } catch (reason) {
    error.value = `节点测试失败：${messageOf(reason)}`
  } finally {
    testingProxyID.value = ''
  }
}

export async function speedTestAllProxies() {
  testingAllProxies.value = true
  error.value = ''
  try {
    const results = await SpeedTestProxyNodes(getBootstrapDNSServer())
    const next = { ...proxyTests.value }
    for (const result of results) next[result.node_id] = result
    proxyTests.value = next
    notice.value = `测速完成：${results.filter(item => item.available).length}/${results.length} 个节点可用。`
  } catch (reason) {
    error.value = `批量测速失败：${messageOf(reason)}`
  } finally {
    testingAllProxies.value = false
  }
}

export async function toggleProxyFavorite(node: nodes.Node) {
  try {
    await SetProxyNodeFavorite(node.id, !node.favorite)
    await refreshProxyState()
  } catch (reason) {
    error.value = `无法更新收藏：${messageOf(reason)}`
  }
}

export async function toggleProxyNodeEgress(node: nodes.Node, isRunning = false) {
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

export function resetSubscriptionForm() {
  subscriptionForm.value = { id: '', name: '', url: '', egress: 'b' }
  subscriptionFormOpen.value = false
}

export function editSubscription(item: subscriptions.Subscription) {
  subscriptionForm.value = { id: item.id, name: item.name, url: '', egress: (item.egress as any) || 'b' }
  subscriptionFormOpen.value = true
}

export async function saveSubscription() {
  busy.value = true
  error.value = ''
  try {
    if (subscriptionForm.value.id) await UpdateSubscription(subscriptionForm.value)
    else await AddSubscription(subscriptionForm.value)
    subscriptionList.value = await ListSubscriptions()
    resetSubscriptionForm()
    await refreshProxyState()
    notice.value = '订阅定义已安全保存。'
  } catch (reason) {
    error.value = `无法保存订阅：${messageOf(reason)}`
  } finally {
    busy.value = false
  }
}

export async function refreshSubscription(item: subscriptions.Subscription) {
  refreshingSubscriptionID.value = item.id
  error.value = ''
  try {
    await RefreshSubscription(item.id)
    subscriptionList.value = await ListSubscriptions()
    await refreshProxyState()
    notice.value = '订阅已校验并原子更新。'
  } catch (reason) {
    subscriptionList.value = await ListSubscriptions()
    error.value = `订阅更新失败，已保留原节点：${messageOf(reason)}`
  } finally {
    refreshingSubscriptionID.value = ''
  }
}

export async function deleteSubscription(item: subscriptions.Subscription) {
  if (!window.confirm(`删除订阅“${item.name}”及其节点？`)) return
  try {
    await DeleteSubscription(item.id)
    subscriptionList.value = await ListSubscriptions()
    await refreshProxyState()
    notice.value = '订阅及其节点已删除。'
  } catch (reason) {
    error.value = `无法删除订阅：${messageOf(reason)}`
  }
}
