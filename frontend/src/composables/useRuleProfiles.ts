import { computed, ref, watch } from 'vue'
import {
  GetRuleProfile,
  SetRuleProfile,
} from '../../wailsjs/go/app/App'
import type { rulesettings, srssets } from '../types'
import { error, messageOf } from './useFeedback'
import { routingMode } from './useNetworkInterfaces'

export const selectedRuleMode = ref<'single' | 'dual'>(routingMode.value || 'dual')

// Watch routingMode and keep selectedRuleMode in sync when not user-overridden
watch(routingMode, newMode => {
  if (newMode === 'single' || newMode === 'dual') {
    selectedRuleMode.value = newMode
  }
})

export const ruleProfiles = ref<Record<'single' | 'dual', rulesettings.Profile>>({
  single: {
    default_outbound: 'a',
    rule_update_outbound: 'auto',
    rules: [],
    rule_order: ['default-private-lan'],
    srs_actions: {},
    srs_enabled: {},
  } as unknown as rulesettings.Profile,
  dual: {
    default_outbound: 'b',
    rule_update_outbound: 'auto',
    rules: [],
    rule_order: ['default-private-lan'],
    srs_actions: {},
    srs_enabled: {},
  } as unknown as rulesettings.Profile,
})

export const currentProfile = computed<rulesettings.Profile>(() => {
  return ruleProfiles.value[selectedRuleMode.value] || ruleProfiles.value.dual
})

export const defaultOutbound = computed<'a' | 'b' | 'c'>({
  get: () => (currentProfile.value.default_outbound as 'a' | 'b' | 'c') || (selectedRuleMode.value === 'single' ? 'a' : 'b'),
  set: (val) => {
    currentProfile.value.default_outbound = val
    void saveRuleSettings()
  },
})

export const ruleUpdateOutbound = computed<'auto' | 'a' | 'b' | 'c'>({
  get: () => (currentProfile.value.rule_update_outbound as 'auto' | 'a' | 'b' | 'c') || 'auto',
  set: (val) => {
    currentProfile.value.rule_update_outbound = val
    void saveRuleSettings()
  },
})

export function updateDefaultOutbound(value: 'a' | 'b' | 'c') {
  defaultOutbound.value = value
}

export function updateRuleUpdateOutbound(value: 'auto' | 'a' | 'b' | 'c') {
  ruleUpdateOutbound.value = value
}

export function sanitizeProfile(prof: rulesettings.Profile, mode: 'single' | 'dual'): rulesettings.Profile {
  if (!prof.rules) prof.rules = []
  if (!prof.rule_order) prof.rule_order = []
  if (!prof.srs_actions) prof.srs_actions = {}
  if (!prof.srs_enabled) prof.srs_enabled = {}
  if (!prof.default_outbound) prof.default_outbound = mode === 'single' ? 'a' : 'b'
  if (!prof.rule_update_outbound) prof.rule_update_outbound = 'auto'
  return prof
}

export function reconcileProfileRuleOrder(profile: rulesettings.Profile, srsSourceList: srssets.Source[]) {
  if (!profile.rule_order) profile.rule_order = []
  if (!profile.rules) profile.rules = []

  for (const source of srsSourceList) {
    if (!profile.rule_order.includes(`srs:${source.id}`)) {
      profile.rule_order.push(`srs:${source.id}`)
    }
  }
  const validRuleKeys = new Set([
    ...profile.rules.map(rule => rule.id),
    ...srsSourceList.map(source => `srs:${source.id}`),
  ])
  profile.rule_order = profile.rule_order.filter(key => validRuleKeys.has(key))
  if (profile.rule_order.includes('default-private-lan')) {
    profile.rule_order = ['default-private-lan', ...profile.rule_order.filter(key => key !== 'default-private-lan')]
  } else if (profile.rules.some(rule => rule.id === 'default-private-lan')) {
    profile.rule_order.unshift('default-private-lan')
  }
}

export async function loadRuleProfiles(): Promise<void> {
  try {
    const [single, dual] = await Promise.all([
      GetRuleProfile('single'),
      GetRuleProfile('dual'),
    ])
    ruleProfiles.value.single = sanitizeProfile(single, 'single')
    ruleProfiles.value.dual = sanitizeProfile(dual, 'dual')
    if (routingMode.value === 'single' || routingMode.value === 'dual') {
      selectedRuleMode.value = routingMode.value
    }
  } catch (reason) {
    error.value = `无法加载规则 Profile：${messageOf(reason)}`
  }
}

let ruleSettingsSave = Promise.resolve()
let ruleSettingsSaveError: unknown

export function saveRuleSettings(): Promise<void> {
  const mode = selectedRuleMode.value
  const profile = currentProfile.value

  ruleSettingsSave = ruleSettingsSave
    .then(async () => {
      const saved = await SetRuleProfile(mode, profile)
      ruleProfiles.value[mode] = sanitizeProfile(saved, mode)
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

type RuleModeChangeListener = (mode: 'single' | 'dual') => void
const changeListeners: RuleModeChangeListener[] = []

export function onRuleModeChange(listener: RuleModeChangeListener) {
  changeListeners.push(listener)
}

export function switchRuleMode(mode: 'single' | 'dual') {
  selectedRuleMode.value = mode
  for (const fn of changeListeners) {
    try {
      fn(mode)
    } catch (e) {
      console.error(e)
    }
  }
}
