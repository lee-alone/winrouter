import { computed, ref } from 'vue'
import type { CustomRule, InlineRuleType, MasterRow, RuleAction, RuleForm } from '../types'
import {
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
import {
  currentProfile,
  defaultOutbound,
  onRuleModeChange,
  saveRuleSettings,
  selectedRuleMode,
} from './useRuleProfiles'
import { resetSRSForm, srsSources, submitSRSSource } from './useSRSRules'


onRuleModeChange(() => {
  resetRuleForm()
  void refreshRulePreview()
})

export const customRules = computed<CustomRule[]>(() => {
  return (currentProfile.value.rules || []) as CustomRule[]
})

export const ruleOrder = computed<string[]>({
  get: () => currentProfile.value.rule_order || [],
  set: (val) => {
    currentProfile.value.rule_order = val
  },
})

export const ruleFormOpen = ref(false)
export const ruleForm = ref<RuleForm>({ id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true })


export const masterRows = computed<MasterRow[]>(() => {
  const rows: MasterRow[] = []
  const seen = new Set<string>()
  const profile = currentProfile.value
  const isSingle = selectedRuleMode.value === 'single'

  for (const key of profile.rule_order || []) {
    const rule = (profile.rules || []).find(item => item.id === key)
    if (rule) {
      rows.push({ key, kind: 'custom', rule: rule as CustomRule })
      seen.add(key)
      continue
    }
    const source = srsSources.value.find(item => `srs:${item.id}` === key)
    if (source) {
      const effectiveAction = profile.srs_actions?.[source.id] || (isSingle && source.action === 'b' ? 'a' : source.action)
      const effectiveEnabled = profile.srs_enabled?.[source.id] ?? source.enabled
      rows.push({
        key,
        kind: 'srs',
        source: {
          ...source,
          action: effectiveAction,
          enabled: effectiveEnabled,
        },
      })
      seen.add(key)
    }
  }

  for (const rule of profile.rules || []) {
    if (!seen.has(rule.id)) {
      rows.push({ key: rule.id, kind: 'custom', rule: rule as CustomRule })
    }
  }

  for (const source of srsSources.value) {
    if (!seen.has(`srs:${source.id}`)) {
      const effectiveAction = profile.srs_actions?.[source.id] || (isSingle && source.action === 'b' ? 'a' : source.action)
      const effectiveEnabled = profile.srs_enabled?.[source.id] ?? source.enabled
      rows.push({
        key: `srs:${source.id}`,
        kind: 'srs',
        source: {
          ...source,
          action: effectiveAction,
          enabled: effectiveEnabled,
        },
      })
    }
  }

  return rows
})


export function resetRuleForm() {
  ruleForm.value = {
    id: '',
    name: '',
    type: 'domain-suffix',
    valuesText: '',
    action: 'a',
    enabled: true,
  }
  resetSRSForm()
  ruleFormOpen.value = false
}

export function editCustomRule(rule: CustomRule) {
  ruleForm.value = { ...rule, valuesText: rule.values.join('\n') }
  ruleFormOpen.value = true
}

export function moveMasterRule(key: string, direction: -1 | 1) {
  const order = currentProfile.value.rule_order
  const index = order.indexOf(key)
  const target = index + direction
  if (index < 0 || target < 0 || target >= order.length) return
  const next = [...order]
  const [item] = next.splice(index, 1)
  next.splice(target, 0, item)
  currentProfile.value.rule_order = next
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
  const rules = currentProfile.value.rules as CustomRule[]
  const index = rules.findIndex(rule => rule.id === next.id)
  next.enabled = next.enabled !== false
  if (index >= 0) rules[index] = next
  else rules.push(next)
  if (!currentProfile.value.rule_order.includes(next.id)) {
    currentProfile.value.rule_order.push(next.id)
  }
  await saveRuleSettings()
  resetRuleForm()
  await refreshRulePreview()
}

export async function deleteCustomRule(id: string) {
  if (id === 'default-private-lan') return
  currentProfile.value.rules = (currentProfile.value.rules as CustomRule[]).filter(rule => rule.id !== id)
  currentProfile.value.rule_order = currentProfile.value.rule_order.filter(key => key !== id)
  await saveRuleSettings()
  await refreshRulePreview()
}

export function formatRuleAction(action: RuleAction): string {
  switch (action) {
    case 'a':
      return selectedRuleMode.value === 'single'
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
      return `跟随默认 (${defaultOutbound.value === 'a' && selectedRuleMode.value === 'single' ? '直连' : defaultOutbound.value.toUpperCase()})`
    default:
      return action
  }
}
