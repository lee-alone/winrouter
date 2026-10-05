export const ssMethods = [
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
]

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

export interface ParsedProxyResult {
  name: string
  type: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  server: string
  port: number
  username?: string
  password?: string
  method?: string
  uuid?: string
  flow?: string
  tls_enabled?: boolean
  tls_server_name?: string
  tls_insecure?: boolean
  tls_alpn?: string
  transport_type?: 'tcp' | 'ws'
  transport_path?: string
  transport_host?: string
}

export function parseProxyURI(uri: string): ParsedProxyResult {
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
      type: 'shadowsocks',
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
      type: 'vmess',
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
      type: 'vless',
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
      type: 'trojan',
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
      type: 'http',
      server: url.hostname,
      port: parseInt(url.port, 10) || 8080,
      username: url.username ? decodeURIComponent(url.username) : '',
      password: url.password ? decodeURIComponent(url.password) : '',
    }
  }

  throw new Error('不支持的链接协议，支持 ss://, vmess://, vless://, trojan://, http://')
}
