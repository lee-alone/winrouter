import { ref } from 'vue'
import {
  AddProxyNode,
  UpdateProxyNode,
} from '../../wailsjs/go/app/App'
import type { nodes } from '../../wailsjs/go/models'
import { parseProxyURI, ssMethods } from '../utils/proxyUri'
import { busy, error, messageOf, notice } from './useFeedback'
import { proxyProtocols, refreshProxyState } from './useProxyNodes'

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

export function openImportBox(): void {
  proxyImportURI.value = ''
  proxyImportError.value = ''
  proxyImportOpen.value = true
}

export function closeImportBox(): void {
  proxyImportOpen.value = false
  proxyImportURI.value = ''
  proxyImportError.value = ''
}

export function openNewProxyForm(): void {
  resetProxyForm()
  proxyFormOpen.value = true
}

export function selectProtocol(type: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'): void {
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

export function resetProxyForm(): void {
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

export function editProxyNode(node: nodes.Node): void {
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

export async function saveProxyNode(isRunning = false): Promise<void> {
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

export function applyImportURI(): void {
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
      username: parsed.username || '',
      password: parsed.password || '',
      method: parsed.method || 'aes-256-gcm',
      uuid: parsed.uuid || '',
      flow: parsed.flow || '',
      tls_enabled: parsed.tls_enabled || parsed.type === 'trojan',
      tls_server_name: parsed.tls_server_name || '',
      tls_insecure: parsed.tls_insecure || false,
      tls_alpn: parsed.tls_alpn || '',
      transport_type: parsed.transport_type || 'tcp',
      transport_path: parsed.transport_path || '',
      transport_host: parsed.transport_host || '',
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
