import { ref } from 'vue'
import {
  InspectProcessRules,
  PreviewCoreRules,
} from '../../wailsjs/go/app/App'
import type { config, processrules } from '../../wailsjs/go/models'
import {
  customRules,
  defaultOutbound,
  flushRuleSettings,
  ruleOrder,
} from './useCustomRules'
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
