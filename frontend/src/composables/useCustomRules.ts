import { computed, ref } from 'vue'
import { SetRuleSettings } from '../../wailsjs/go/app/App'
import type { rulesettings } from '../../wailsjs/go/models'
import type { CustomRule, InlineRuleType, MasterRow, RuleAction, RuleForm } from '../types'
import { error, messageOf } from './useFeedback'
import {
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
} from './useNetworkInterfaces'
import {
  chainSummaryText,
  isChainMode,
  isChainReady,
  proxyNodes,
} from './useProxyManager'
import { refreshRulePreview } from './useRulePreview'
import { resetSRSForm, srsSources, submitSRSSource } from './useSRSRules'

export const defaultOutbound = ref<'a' | 'b' | 'c'>('b')
export const ruleUpdateOutbound = ref<'auto' | 'a' | 'b' | 'c'>('auto')
export const customRules = ref<CustomRule[]>([])
export const ruleOrder = ref<string[]>([])
export const ruleFormOpen = ref(false)
export const ruleForm = ref<RuleForm>({ id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true })

let ruleSettingsSave = Promise.resolve()
let ruleSettingsSaveError: unknown

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
