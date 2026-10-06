import { computed, ref } from 'vue'
import {
  ConfigureSRSSource,
  DeleteSRSSource,
  ListSRSSources,
  RefreshSRSSource,
} from '../../wailsjs/go/app/App'
import type { srssets } from '../../wailsjs/go/models'
import type { RuleAction } from '../types'
import {
  resetRuleForm,
  ruleForm,
  ruleFormOpen,
  ruleOrder,
} from './useCustomRules'
import { error, messageOf, notice } from './useFeedback'
import { refreshRulePreview } from './useRulePreview'
import {
  currentProfile,
  ruleProfiles,
  saveRuleSettings,
  selectedRuleMode,
} from './useRuleProfiles'

export const srsPresets = ref<srssets.Preset[]>([])
export const srsSources = ref<srssets.Source[]>([])
export const srsBusyID = ref('')
export const srsForm = ref({
  id: '',
  name: '',
  kind: 'domain',
  preset_id: '',
  url: '',
  expected_sha256: '',
  enabled: true,
  action: 'a' as RuleAction,
})

export const geoRulesNotReady = computed(() => srsSources.value.filter(source => source.enabled && !source.applied_sha256))

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
    const profile = currentProfile.value
    if (!profile.rule_order) profile.rule_order = []
    if (!profile.rule_order.includes(key)) profile.rule_order.push(key)
    if (!profile.srs_actions) profile.srs_actions = {}
    if (!profile.srs_enabled) profile.srs_enabled = {}
    profile.srs_actions[configured.id] = ruleForm.value.action
    profile.srs_enabled[configured.id] = ruleForm.value.enabled

    const otherMode = selectedRuleMode.value === 'single' ? 'dual' : 'single'
    const otherProf = ruleProfiles.value[otherMode]
    if (otherProf) {
      if (!otherProf.rule_order) otherProf.rule_order = []
      if (!otherProf.rule_order.includes(key)) otherProf.rule_order.push(key)
    }

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
    const profile = currentProfile.value
    if (!profile.srs_enabled) profile.srs_enabled = {}
    const nextEnabled = !source.enabled
    profile.srs_enabled[source.id] = nextEnabled
    await saveRuleSettings()
    notice.value = `${source.name} 已在${selectedRuleMode.value === 'single' ? '单网卡' : '双网卡'}模式中${nextEnabled ? '启用' : '停用'}。`
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
    const key = `srs:${source.id}`
    for (const mode of ['single', 'dual'] as const) {
      const prof = ruleProfiles.value[mode]
      if (prof) {
        if (prof.rule_order) prof.rule_order = prof.rule_order.filter(k => k !== key)
        if (prof.srs_actions) delete prof.srs_actions[source.id]
        if (prof.srs_enabled) delete prof.srs_enabled[source.id]
      }
    }
    await saveRuleSettings()
    await refreshRulePreview()
    notice.value = '自定义规则集已删除。'
  } catch (reason) {
    error.value = `无法删除 SRS 来源：${messageOf(reason)}`
  }
}
