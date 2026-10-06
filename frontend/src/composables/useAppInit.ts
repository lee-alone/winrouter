import {
  GetApplicationConfigDirectory,
  GetApplicationConfigInfo,
  GetAutostartStatus,
  GetCoreStatus,
  GetDNSPresets,
  GetDNSSettings,
  GetInterfaceSnapshot,
  GetIPv6Policy,
  GetObservations,
  GetRecoveryStatus,
  GetSRSPresets,
  GetStatus,
  GetTrafficBudgetStatus,
  ListSRSSources,
  ListSubscriptions,
} from '../../wailsjs/go/app/App'
import type { core } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import type { CustomRule, DNSPreset, DNSSettings, RecoveryStatus } from '../types'
import { addLog } from './useAppLogs'
import {
  applicationConfigDirectory,
  autostartEnabled,
  budgetForm,
  initializationFailed,
  isPortableConfig,
} from './useAppSettings'
import {
  app,
  coreStatus,
  isRunning,
  recoveryStatus,
  startCore,
} from './useCoreManager'
import {
  dnsPresets,
  dnsSettings,
  normalizeDNSSettings,
} from './useDNSManager'
import { busy, error, messageOf } from './useFeedback'
import {
  ipv6Policy,
  syncSelection,
} from './useNetworkInterfaces'
import {
  formatBytes,
  normalizeObservations,
  observations,
  observationsError,
  sampleTraffic,
  usageBaselines,
} from './useObservability'
import {
  refreshProxyState,
  subscriptionList,
} from './useProxyManager'
import {
  loadRuleProfiles,
  reconcileProfileRuleOrder,
  ruleProfiles,
  srsPresets,
  srsSources,
} from './useRulesManager'
import {
  loadSecurityStatus,
  openPinModal,
  securityStatus,
} from './useSecurityVault'

let stopEvents: (() => void) | undefined
let stopRecoveryEvents: (() => void) | undefined
let stopTrayStartEvents: (() => void) | undefined
let stopBudgetEvents: (() => void) | undefined
let statusTimer: number | undefined

export async function initializeApp() {
  try {
    const cfgInfo = await GetApplicationConfigInfo()
    if (cfgInfo && cfgInfo.path) {
      applicationConfigDirectory.value = cfgInfo.path
      isPortableConfig.value = cfgInfo.is_portable
    } else {
      applicationConfigDirectory.value = await GetApplicationConfigDirectory()
    }
  } catch {
    applicationConfigDirectory.value = ''
  }

  await loadSecurityStatus()
  if (securityStatus.value.pin_enabled && !securityStatus.value.unlocked) {
    openPinModal('unlock')
  }

  try {
    const storedBaselines = JSON.parse(localStorage.getItem('winrouter.trafficBaselines.v1') || '{}')
    if (storedBaselines && typeof storedBaselines === 'object' && !Array.isArray(storedBaselines)) {
      usageBaselines.value = storedBaselines
    }
  } catch {
    localStorage.removeItem('winrouter.trafficBaselines.v1')
  }

  try {
    const [
      appStatus,
      interfaceSnapshot,
      status,
      recovery,
      observed,
      autostart,
      budget,
      storedDNS,
      presets,
      storedIPv6,
    ] = await Promise.all([
      GetStatus(),
      GetInterfaceSnapshot(),
      GetCoreStatus(),
      GetRecoveryStatus(),
      GetObservations(),
      GetAutostartStatus(),
      GetTrafficBudgetStatus(),
      GetDNSSettings(),
      GetDNSPresets(),
      GetIPv6Policy(),
    ])

    app.value = appStatus
    syncSelection(interfaceSnapshot)
    coreStatus.value = status
    recoveryStatus.value = recovery as RecoveryStatus
    observations.value = normalizeObservations(observed)
    observationsError.value = ''
    sampleTraffic(observations.value.counters)
    autostartEnabled.value = autostart.enabled
    budgetForm.value = { enabled: budget.enabled, budget_gb: budget.budget_gb, warning_percent: budget.warning_percent }
    dnsSettings.value = normalizeDNSSettings(storedDNS as DNSSettings)
    dnsPresets.value = presets as DNSPreset[]
    ipv6Policy.value = storedIPv6 as 'block' | 'split'

    await refreshProxyState()
    subscriptionList.value = await ListSubscriptions()
    srsPresets.value = await GetSRSPresets()
    srsSources.value = await ListSRSSources()

    await loadRuleProfiles()
    for (const mode of ['single', 'dual'] as const) {
      reconcileProfileRuleOrder(ruleProfiles.value[mode], srsSources.value)
    }

    addLog('info', `应用已就绪，发现 ${interfaceSnapshot.candidates.length} 块候选网卡。`)
  } catch (reason) {
    initializationFailed.value = true
    error.value = `初始化失败：${messageOf(reason)}`
  }

  stopEvents = EventsOn('core-status', payload => {
    coreStatus.value = payload as core.Status
    if (coreStatus.value.abnormal) addLog('error', `核心异常：${coreStatus.value.last_error || '未知错误'}`, true)
  })

  stopRecoveryEvents = EventsOn('recovery-status', payload => {
    recoveryStatus.value = payload as RecoveryStatus
    if (recoveryStatus.value.state === 'failed') addLog('error', `网络恢复失败：${recoveryStatus.value.last_error}`, true)
  })

  stopTrayStartEvents = EventsOn('tray-start-requested', () => {
    if (!isRunning.value && !busy.value) void startCore()
  })

  stopBudgetEvents = EventsOn('traffic-budget-warning', payload => {
    const data = payload as { used_bytes: number; budget_gb: number; used_percent: number }
    addLog(
      'warning',
      `网卡 B 流量已达到预算 ${data.used_percent.toFixed(1)}%（${formatBytes(data.used_bytes)} / ${data.budget_gb} GB）`
    )
  })

  statusTimer = window.setInterval(async () => {
    try {
      const [status, recovery, observed] = await Promise.all([
        GetCoreStatus(),
        GetRecoveryStatus(),
        GetObservations(),
      ])
      coreStatus.value = status
      recoveryStatus.value = recovery as RecoveryStatus
      observations.value = normalizeObservations(observed)
      observationsError.value = ''
      sampleTraffic(observations.value.counters)
    } catch (reason) {
      observationsError.value = messageOf(reason)
    }
  }, 1000)
}

export function cleanupApp() {
  stopEvents?.()
  stopRecoveryEvents?.()
  stopTrayStartEvents?.()
  stopBudgetEvents?.()
  if (statusTimer) window.clearInterval(statusTimer)
}
