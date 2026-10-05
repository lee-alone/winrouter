import { ref } from 'vue'
import { SetDNSSettings, TestDNSServer } from '../../wailsjs/go/app/App'
import type { DNSPreset, DNSServer, DNSSettings, DNSTestState } from '../types'
import { error, messageOf, notice } from './useFeedback'

export function normalizeDNSSettings(value: DNSSettings): DNSSettings {
  const normalizeServer = (server: DNSServer): DNSServer => ({
    ...server,
    preset_id: server.preset_id ?? '',
    server_name: server.server_name ?? '',
  })
  return {
    ...value,
    domestic: normalizeServer(value.domestic),
    global: normalizeServer(value.global),
    proxy: value.proxy
      ? normalizeServer(value.proxy)
      : { preset_id: 'proxy-cloudflare-doh', type: 'https', server: '1.1.1.1', port: 443, server_name: 'cloudflare-dns.com' },
  }
}

export const dnsSettings = ref<DNSSettings>({
  schema_version: 1,
  domestic: { preset_id: 'aliyun-dot', type: 'tls', server: '223.5.5.5', port: 853, server_name: 'dns.alidns.com' },
  global: { preset_id: 'tencent-dot', type: 'tls', server: '1.12.12.12', port: 853, server_name: 'dot.pub' },
  proxy: { preset_id: 'proxy-cloudflare-doh', type: 'https', server: '1.1.1.1', port: 443, server_name: 'cloudflare-dns.com' },
})

export const dnsPresets = ref<DNSPreset[]>([])
export const dnsBusy = ref(false)
export const dnsTests = ref<Record<'domestic' | 'global' | 'proxy', DNSTestState>>({
  domestic: 'idle',
  global: 'idle',
  proxy: 'idle',
})

export function chooseDNSPreset(scope: 'domestic' | 'global' | 'proxy') {
  const target = dnsSettings.value[scope]
  if (!target || !target.preset_id) return
  const preset = dnsPresets.value.find(item => item.id === target.preset_id)
  if (preset) {
    dnsSettings.value[scope] = {
      preset_id: preset.id,
      type: preset.type,
      server: preset.server,
      port: preset.port,
      server_name: preset.server_name || '',
    }
  }
  dnsTests.value[scope] = 'idle'
}

export function setCustomDNS(scope: 'domestic' | 'global' | 'proxy') {
  if (dnsSettings.value[scope]) {
    dnsSettings.value[scope]!.preset_id = ''
  }
  dnsTests.value[scope] = 'idle'
}

export async function testDNSServer(scope: 'domestic' | 'global' | 'proxy') {
  const target = dnsSettings.value[scope]
  if (!target) return
  dnsTests.value[scope] = 'testing'
  try {
    const result = (await TestDNSServer({ ...target } as any)) as { success: boolean }
    dnsTests.value[scope] = result.success ? 'success' : 'failed'
  } catch {
    dnsTests.value[scope] = 'failed'
  }
}

export async function saveDNSSettings(onSaved?: () => Promise<void> | void) {
  dnsBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    dnsSettings.value = normalizeDNSSettings((await SetDNSSettings(dnsSettings.value as any)) as DNSSettings)
    notice.value = 'DNS 设置已保存，将用于预览、启动和网络恢复。'
    if (onSaved) await onSaved()
  } catch (reason) {
    error.value = `无法保存 DNS 设置：${messageOf(reason)}`
  } finally {
    dnsBusy.value = false
  }
}

export function getBootstrapDNSServer(): string {
  return dnsSettings.value.domestic?.server || '223.5.5.5'
}
