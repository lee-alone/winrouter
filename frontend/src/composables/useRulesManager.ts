import { computed, ref } from 'vue'
import {
  ConfigureSRSSource,
  DeleteSRSSource,
  InspectProcessRules,
  ListSRSSources,
  PreviewCoreRules,
  RefreshSRSSource,
  SetRuleSettings,
} from '../../wailsjs/go/app/App'
import type { config, processrules, rulesettings, srssets } from '../../wailsjs/go/models'
import type { CustomRule, InlineRuleType, MasterRow, RuleAction, RuleForm } from '../types'
import { dnsSettings } from './useDNSManager'
import { error, messageOf, notice } from './useFeedback'
import {
  directPrefixes,
  ipv6Policy,
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
  snapshot,
} from './useNetworkInterfaces'
import {
  chainSummaryText,
  isChainMode,
  isChainReady,
  proxyNodes,
} from './useProxyManager'

export const defaultOutbound = ref<'a' | 'b' | 'c'>('b')
export const ruleUpdateOutbound = ref<'auto' | 'a' | 'b' | 'c'>('auto')
export const customRules = ref<CustomRule[]>([])
export const ruleOrder = ref<string[]>([])
export const ruleFormOpen = ref(false)
export const ruleForm = ref<RuleForm>({ id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true })
export const rulePreview = ref<config.RulePreview[]>([])
export const rulePreviewError = ref('')
export const processRuleStatuses = ref<processrules.Status[]>([])
export const srsPresets = ref<srssets.Preset[]>([])
export const srsSources = ref<srssets.Source[]>([])
export const srsBusyID = ref('')
export const srsForm = ref({ id: '', name: '', kind: 'domain', preset_id: '', url: '', expected_sha256: '', enabled: true, action: 'a' as RuleAction })

let ruleSettingsSave = Promise.resolve()
let ruleSettingsSaveError: unknown

export const geoRulesNotReady = computed(() => srsSources.value.filter(source => source.enabled && !source.applied_sha256))

export const masterRows = computed<MasterRow[]>(() => {
  const rows: MasterRow[] = []
  const seen = new Set<string>()
  for (const key of ruleOrder.value) {
    const rule = customRules.value.find(item => item.id === key)
    if (rule) {
      rows.push({ key, kind: 'custom', rule })
      seen.add(key)
      continue
    }
    const source = srsSources.value.find(item => `srs:${item.id}` === key)
    if (source) {
      rows.push({ key, kind: 'srs', source })
      seen.add(key)
    }
  }
  for (const rule of customRules.value) if (!seen.has(rule.id)) rows.push({ key: rule.id, kind: 'custom', rule })
  for (const source of srsSources.value) if (!seen.has(`srs:${source.id}`)) rows.push({ key: `srs:${source.id}`, kind: 'srs', source })
  return rows
})

export function formatSRSUpdatedAt(source: srssets.Source): string {
  if (!source.updated_at) return '尚未成功更新'
  const date = new Date(source.updated_at)
  if (Number.isNaN(date.getTime())) return '更新时间未知'
  return `最近更新：${date.toLocaleString('zh-CN', { hour12: false })}`
}

export function resetSRSForm() {
  srsForm.value = { id: '', name: '', kind: 'domain', preset_id: '', url: '', expected_sha256: '', enabled: true, action: 'a' }
}

export function chooseSRSPreset() {
  const preset = srsPresets.value.find(item => item.id === srsForm.value.preset_id)
  if (!preset) return
  srsForm.value.name = preset.name
  ruleForm.value.name = preset.name
  srsForm.value.kind = preset.kind
  srsForm.value.url = preset.url
  srsForm.value.expected_sha256 = ''
}

export function editSRSSource(source: srssets.Source) {
  srsForm.value = {
    id: source.id,
    name: source.name,
    kind: source.kind,
    preset_id: source.preset_id || '',
    url: source.url,
    expected_sha256: source.expected_sha256 || '',
    enabled: source.enabled,
    action: source.action as RuleAction,
  }
  ruleForm.value = {
    id: source.id,
    name: source.name,
    type: 'rule-set',
    valuesText: '',
    action: source.action as RuleAction,
    enabled: source.enabled,
  }
  ruleFormOpen.value = true
}

export function saveRuleSettings(): Promise<void> {
  const snapshotData = {
    schema_version: 2,
    initialized: true,
    default_outbound: defaultOutbound.value,
    rule_update_outbound: ruleUpdateOutbound.value,
    rules: customRules.value.map(rule => ({
      id: rule.id,
      name: rule.name,
      type: rule.type,
      values: rule.values,
      action: rule.action,
      enabled: rule.enabled,
    })),
    rule_order: [...ruleOrder.value],
  } as rulesettings.Settings

  ruleSettingsSave = ruleSettingsSave
    .then(async () => {
      await SetRuleSettings(snapshotData)
      ruleSettingsSaveError = undefined
      localStorage.removeItem('winrouter.customRules.v1')
      localStorage.removeItem('winrouter.ruleOrder.v1')
      localStorage.removeItem('winrouter.default-outbound.v1')
    })
    .catch(reason => {
      ruleSettingsSaveError = reason
      error.value = `无法保存规则设置：${messageOf(reason)}`
    })
  return ruleSettingsSave
}

export async function flushRuleSettings(): Promise<void> {
  await ruleSettingsSave
  if (ruleSettingsSaveError) throw ruleSettingsSaveError
}

export function resetRuleForm() {
  ruleForm.value = { id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true }
  resetSRSForm()
  ruleFormOpen.value = false
}

export function editCustomRule(rule: CustomRule) {
  ruleForm.value = { ...rule, valuesText: rule.values.join('\n') }
  ruleFormOpen.value = true
}

export function moveMasterRule(key: string, direction: -1 | 1) {
  const index = ruleOrder.value.indexOf(key)
  const target = index + direction
  if (index < 0 || target < 0 || target >= ruleOrder.value.length) return
  const next = [...ruleOrder.value]
  const [item] = next.splice(index, 1)
  next.splice(target, 0, item)
  ruleOrder.value = next
  void saveRuleSettings().then(refreshRulePreview)
}

export function toggleCustomRule(rule: CustomRule) {
  rule.enabled = !rule.enabled
  void saveRuleSettings().then(refreshRulePreview)
}

export function ruleTypeText(type: InlineRuleType) {
  return ({
    'domain-suffix': '域名后缀',
    domain: '精确域名',
    ip: 'IP/CIDR',
    'process-name': '进程',
    'process-path': '进程路径',
  } as Record<InlineRuleType, string>)[type]
}

export function normalizeRuleValue(type: InlineRuleType, raw: string) {
  const value = raw.trim()
  if (type === 'domain' || type === 'domain-suffix') return value.toLowerCase().replace(/^\.+|\.+$/g, '')
  if (type === 'process-name' || type === 'process-path') return value.toLowerCase()
  return value
}

export function buildConfig(): config.MVPConfig {
  const interfaceA = selectedAdapterA.value
  const isSingle = routingMode.value === 'single'
  const interfaceB = isSingle ? undefined : selectedAdapterB.value
  if (!interfaceA || (!interfaceB && !isSingle) || !snapshot.value?.tun) throw new Error('接口或 TUN 前缀尚未就绪')
  return {
    schema_version: 1,
    mode: routingMode.value,
    tun: { prefix: snapshot.value.tun.prefix, stack: 'system' },
    interface_a: { guid: interfaceA.guid, bind_interface: interfaceA.friendly_name },
    interface_b: interfaceB ? { guid: interfaceB.guid, bind_interface: interfaceB.friendly_name } : { guid: '', bind_interface: '' },
    default_outbound: defaultOutbound.value,
    direct_prefixes: directPrefixes.value.map(item => ({ prefix: item.prefix, bind_interface: item.adapter_name })),
    rule_order: ruleOrder.value,
    custom_rules: customRules.value.filter(rule => rule.enabled).flatMap(rule => rule.values.map(value => ({ id: rule.id, name: rule.name, type: rule.type, value, action: rule.action }))),
    domestic: { cidrs: [], domain_suffixes: [] },
    dns: {
      domestic: { ...dnsSettings.value.domestic },
      global: { ...dnsSettings.value.global },
      proxy: dnsSettings.value.proxy ? { ...dnsSettings.value.proxy } : undefined,
    },
    ipv6: ipv6Policy.value,
  } as unknown as config.MVPConfig
}

export async function refreshRulePreview() {
  rulePreviewError.value = ''
  try {
    await flushRuleSettings()
    const input = buildConfig()
    rulePreview.value = await PreviewCoreRules(input)
    processRuleStatuses.value = await InspectProcessRules(input)
  } catch (reason) {
    rulePreview.value = []
    processRuleStatuses.value = []
    rulePreviewError.value = messageOf(reason)
  }
}

export async function submitSRSSource() {
  srsBusyID.value = srsForm.value.id || 'new'
  error.value = ''
  notice.value = ''
  try {
    const configured = await ConfigureSRSSource({
      ...srsForm.value,
      name: ruleForm.value.name,
      action: ruleForm.value.action,
      enabled: ruleForm.value.enabled,
      applied_sha256: '',
      size: 0,
      last_error: '',
      upstream: '',
      license: '',
    } as srssets.Source)
    const key = `srs:${configured.id}`
    if (!ruleOrder.value.includes(key)) ruleOrder.value.push(key)
    srsSources.value = await ListSRSSources()
    await saveRuleSettings()
    notice.value = '规则集已保存。下载并验证成功前不会参与分流。'
    resetRuleForm()
    await refreshRulePreview()
  } catch (reason) {
    error.value = `无法保存 SRS 来源：${messageOf(reason)}`
  } finally {
    srsBusyID.value = ''
  }
}

export async function refreshSRSSource(source: srssets.Source) {
  srsBusyID.value = source.id
  error.value = ''
  notice.value = ''
  try {
    await RefreshSRSSource(source.id)
    srsSources.value = await ListSRSSources()
    notice.value = `${source.name} 已下载、校验并缓存。`
    await refreshRulePreview()
  } catch (reason) {
    srsSources.value = await ListSRSSources()
    error.value = `SRS 更新失败，已保留最后有效版本：${messageOf(reason)}`
  } finally {
    srsBusyID.value = ''
  }
}

export async function toggleSRSSource(source: srssets.Source) {
  srsBusyID.value = source.id
  error.value = ''
  notice.value = ''
  try {
    await ConfigureSRSSource({ ...source, enabled: !source.enabled } as srssets.Source)
    srsSources.value = await ListSRSSources()
    notice.value = `${source.name} 已${source.enabled ? '停用' : '启用'}。`
    await refreshRulePreview()
  } catch (reason) {
    error.value = `无法更新规则状态：${messageOf(reason)}`
  } finally {
    srsBusyID.value = ''
  }
}

export async function deleteSRSSource(source: srssets.Source) {
  if (!window.confirm(`删除 SRS 来源“${source.name}”？`)) return
  try {
    await DeleteSRSSource(source.id)
    srsSources.value = await ListSRSSources()
    ruleOrder.value = ruleOrder.value.filter(key => key !== `srs:${source.id}`)
    await saveRuleSettings()
    await refreshRulePreview()
    notice.value = '自定义规则集已删除。'
  } catch (reason) {
    error.value = `无法删除 SRS 来源：${messageOf(reason)}`
  }
}

export async function submitRule() {
  if (ruleForm.value.type === 'rule-set') {
    await submitSRSSource()
    return
  }
  const type = ruleForm.value.type as InlineRuleType
  const values = ruleForm.value.valuesText
    .split(/\r?\n/)
    .map(value => normalizeRuleValue(type, value))
    .filter(Boolean)
  if (!values.length) throw new Error('至少填写一个匹配值')
  const next: CustomRule = {
    id: ruleForm.value.id || crypto.randomUUID(),
    name: ruleForm.value.name,
    type,
    values: [...new Set(values)],
    action: ruleForm.value.action,
    enabled: ruleForm.value.enabled,
  }
  const index = customRules.value.findIndex(rule => rule.id === next.id)
  next.enabled = next.enabled !== false
  if (index >= 0) customRules.value[index] = next
  else customRules.value.push(next)
  if (!ruleOrder.value.includes(next.id)) ruleOrder.value.push(next.id)
  await saveRuleSettings()
  resetRuleForm()
  await refreshRulePreview()
}

export async function deleteCustomRule(id: string) {
  if (id === 'default-private-lan') return
  customRules.value = customRules.value.filter(rule => rule.id !== id)
  ruleOrder.value = ruleOrder.value.filter(key => key !== id)
  await saveRuleSettings()
  await refreshRulePreview()
}

export function updateDefaultOutbound(value: 'a' | 'b' | 'c') {
  defaultOutbound.value = value
  void saveRuleSettings().then(refreshRulePreview)
}

export function updateRuleUpdateOutbound(value: 'auto' | 'a' | 'b' | 'c') {
  ruleUpdateOutbound.value = value
  void saveRuleSettings()
}

export function formatRuleAction(action: RuleAction): string {
  switch (action) {
    case 'a':
      return routingMode.value === 'single'
        ? `直连 · ${selectedAdapterA.value?.friendly_name ?? '主网卡'}`
        : `出口 A · ${selectedAdapterA.value?.friendly_name ?? '网卡 A'}`
    case 'b':
      return `出口 B · ${selectedAdapterB.value?.friendly_name ?? '网卡 B'}`
    case 'c':
      if (isChainMode.value && isChainReady.value) {
        return `出口 C · 链式套接 (${chainSummaryText.value})`
      }
      return `出口 C · 代理 (${proxyNodes.value.find(n => n.selected)?.name ?? '活动节点'})`
    case 'reject':
      return '阻断拒绝'
    case 'final':
      return `跟随默认 (${defaultOutbound.value === 'a' && routingMode.value === 'single' ? '直连' : defaultOutbound.value.toUpperCase()})`
    default:
      return action
  }
}
