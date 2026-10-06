import { ref } from 'vue'
import {
  InspectProcessRules,
  PreviewCoreRules,
} from '../../wailsjs/go/app/App'
import type { config, processrules } from '../types'
import {
  flushRuleSettings,
  ruleProfiles,
  selectedRuleMode,
} from './useRuleProfiles'
import { dnsSettings } from './useDNSManager'
import { messageOf } from './useFeedback'
import {
  directPrefixes,
  ipv6Policy,
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
  snapshot,
} from './useNetworkInterfaces'

export const rulePreview = ref<config.RulePreview[]>([])
export const rulePreviewError = ref('')
export const processRuleStatuses = ref<processrules.Status[]>([])

export function buildConfigForMode(targetMode: 'single' | 'dual'): config.MVPConfig {
  const interfaceA = selectedAdapterA.value
  const isSingle = targetMode === 'single'
  const interfaceB = isSingle ? undefined : selectedAdapterB.value
  if (!interfaceA || (!interfaceB && !isSingle) || !snapshot.value?.tun) {
    throw new Error('接口或 TUN 前缀尚未就绪')
  }

  const profile = ruleProfiles.value[targetMode] || ruleProfiles.value.dual
  const outbound = profile.default_outbound || (isSingle ? 'a' : 'b')
  const order = profile.rule_order || []
  const rules = profile.rules || []

  return {
    schema_version: 1,
    mode: targetMode,
    tun: { prefix: snapshot.value.tun.prefix, stack: 'system' },
    interface_a: { guid: interfaceA.guid, bind_interface: interfaceA.friendly_name },
    interface_b: interfaceB ? { guid: interfaceB.guid, bind_interface: interfaceB.friendly_name } : { guid: '', bind_interface: '' },
    default_outbound: outbound,
    direct_prefixes: directPrefixes.value.map(item => ({ prefix: item.prefix, bind_interface: item.adapter_name })),
    rule_order: order,
    custom_rules: rules
      .filter(rule => rule.enabled)
      .flatMap(rule => rule.values.map(value => ({
        id: rule.id,
        name: rule.name,
        type: rule.type,
        value,
        action: rule.action,
      }))),
    domestic: { cidrs: [], domain_suffixes: [] },
    dns: {
      domestic: { ...dnsSettings.value.domestic },
      global: { ...dnsSettings.value.global },
      proxy: dnsSettings.value.proxy ? { ...dnsSettings.value.proxy } : undefined,
    },
    ipv6: ipv6Policy.value,
  } as unknown as config.MVPConfig
}

// buildConfig is used by startCore to build config strictly matching routingMode configured in network interfaces
export function buildConfig(): config.MVPConfig {
  const mode = (routingMode.value === 'single' ? 'single' : 'dual') as 'single' | 'dual'
  return buildConfigForMode(mode)
}

export async function refreshRulePreview() {
  rulePreviewError.value = ''
  try {
    await flushRuleSettings()
    // For live rules preview, preview the tab currently viewed by the user
    const viewMode = (selectedRuleMode.value === 'single' ? 'single' : 'dual') as 'single' | 'dual'
    const input = buildConfigForMode(viewMode)
    rulePreview.value = await PreviewCoreRules(input)
    processRuleStatuses.value = await InspectProcessRules(input)
  } catch (reason) {
    rulePreview.value = []
    processRuleStatuses.value = []
    rulePreviewError.value = messageOf(reason)
  }
}
