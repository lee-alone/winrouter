import { ref } from 'vue'
import {
  GetDNSSettings,
  GetInterfaceSnapshot,
  GetIPv6Policy,
  GetRuleSettings,
  RepairApplicationSettings,
  ResetApplicationSettings,
  ResetInterfaceSelection,
  ResetTrafficBudget,
  ResetWindowsNetworkStack,
  SetAutostartEnabled,
  SetTrafficBudget,
} from '../../wailsjs/go/app/App'
import type { app as appModels } from '../../wailsjs/go/models'
import type { CustomRule, DNSSettings, TrafficBudgetStatus } from '../types'
import { addLog } from './useAppLogs'
import { coreStatus } from './useCoreManager'
import { dnsSettings, normalizeDNSSettings } from './useDNSManager'
import { error, messageOf, notice } from './useFeedback'
import { setView } from './useNavigation'
import { ipv6Policy, syncSelection } from './useNetworkInterfaces'
import { observations } from './useObservability'
import {
  customRules,
  defaultOutbound,
  ruleOrder,
  ruleUpdateOutbound,
} from './useRulesManager'

export const autostartEnabled = ref(false)
export const autostartBusy = ref(false)
export const budgetBusy = ref(false)
export const budgetForm = ref({ enabled: false, budget_gb: 100, warning_percent: 80 })
export const networkResetBusy = ref(false)
export const networkResetResult = ref<appModels.NetworkResetResult>()
export const applicationResetBusy = ref(false)
export const initializationFailed = ref(false)
export const applicationConfigDirectory = ref('')
export const isPortableConfig = ref(false)

export async function updateAutostart() {
  const requested = autostartEnabled.value
  autostartBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    const status = await SetAutostartEnabled(requested)
    autostartEnabled.value = status.enabled
    notice.value = status.enabled ? '已为当前 Windows 用户启用开机启动。' : '开机启动已关闭。'
    addLog('info', notice.value)
  } catch (reason) {
    autostartEnabled.value = !requested
    error.value = `无法更新开机启动：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally {
    autostartBusy.value = false
  }
}

export async function saveTrafficBudget() {
  budgetBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    const status = (await SetTrafficBudget({ ...budgetForm.value })) as TrafficBudgetStatus
    observations.value.traffic_budget = status
    notice.value = status.enabled ? '网卡 B 月度流量预算提醒已保存。' : '网卡 B 流量预算提醒已停用。'
  } catch (reason) {
    error.value = `无法保存流量预算：${messageOf(reason)}`
  } finally {
    budgetBusy.value = false
  }
}

export async function resetTrafficBudget() {
  if (!window.confirm('将本月已统计的网卡 B 流量归零？此操作不会改变运营商统计。')) return
  budgetBusy.value = true
  try {
    observations.value.traffic_budget = (await ResetTrafficBudget()) as TrafficBudgetStatus
    notice.value = '本月本地流量统计已归零。'
  } catch (reason) {
    error.value = `无法重置流量统计：${messageOf(reason)}`
  } finally {
    budgetBusy.value = false
  }
}

export async function resetWindowsNetworkStack() {
  const warning =
    '此操作将停止分流核心，并重置 Windows Winsock、TCP/IP 和 DNS 缓存。可能清除静态 IP、网关、DNS、VPN 或虚拟网卡配置，执行期间网络会中断，完成后通常必须重启 Windows。是否继续？'
  if (!window.confirm(warning)) return
  if (!window.confirm('最后确认：立即执行系统级网络重置？执行后 WinRouter 不会自动重新启动分流核心。')) return
  networkResetBusy.value = true
  networkResetResult.value = undefined
  error.value = ''
  notice.value = ''
  try {
    const result = (await ResetWindowsNetworkStack()) as appModels.NetworkResetResult
    networkResetResult.value = result
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = result.success
      ? '网络组件重置已完成。请保存工作并重启 Windows。'
      : '网络组件重置未全部成功，请查看各步骤结果后重启 Windows。'
  } catch (reason) {
    error.value = `无法重置 Windows 网络组件：${messageOf(reason)}`
  } finally {
    networkResetBusy.value = false
  }
}

export async function resetSavedInterfaces() {
  if (!window.confirm('清除已保存的出口 A/B 网卡选择？规则、DNS、代理节点和订阅都会保留。分流核心将停止。')) return
  applicationResetBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    syncSelection(await ResetInterfaceSelection())
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = '网卡选择已重置，请重新选择出口 A 和 B。'
    setView('interfaces')
  } catch (reason) {
    error.value = `无法重置网卡选择：${messageOf(reason)}`
  } finally {
    applicationResetBusy.value = false
  }
}

export async function resetApplicationSettings() {
  if (
    !window.confirm(
      '恢复应用默认设置？这会停止核心并重置网卡选择、规则、DNS 和 IPv6 策略。代理节点、订阅、规则集来源及流量统计会保留。'
    )
  )
    return
  if (!window.confirm('重置前会自动备份当前配置。确认继续？')) return
  applicationResetBusy.value = true
  error.value = ''
  notice.value = ''
  try {
    const backup = await ResetApplicationSettings()
    syncSelection(await GetInterfaceSnapshot())
    const storedRules = await GetRuleSettings()
    customRules.value = Array.isArray(storedRules.rules) ? (storedRules.rules as CustomRule[]) : []
    ruleOrder.value = Array.isArray(storedRules.rule_order) ? [...storedRules.rule_order] : []
    defaultOutbound.value = storedRules.default_outbound as 'a' | 'b' | 'c'
    ruleUpdateOutbound.value = (storedRules.rule_update_outbound as 'auto' | 'a' | 'b' | 'c') || 'auto'
    dnsSettings.value = normalizeDNSSettings((await GetDNSSettings()) as DNSSettings)
    ipv6Policy.value = (await GetIPv6Policy()) as 'block' | 'split'
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = `应用设置已恢复默认值。原配置备份于：${backup}`
  } catch (reason) {
    error.value = `无法恢复应用默认设置：${messageOf(reason)}`
  } finally {
    applicationResetBusy.value = false
  }
}

export async function repairApplicationSettings() {
  if (
    !window.confirm(
      '强制修复会备份并重建网卡选择、规则和 DNS 配置文件。代理节点、订阅及规则集来源会保留。修复后必须重启 WinRouter，是否继续？'
    )
  )
    return
  applicationResetBusy.value = true
  notice.value = ''
  try {
    const backup = await RepairApplicationSettings()
    initializationFailed.value = false
    error.value = ''
    notice.value = `配置文件已修复，原文件备份于：${backup}。请退出并重新启动 WinRouter。`
  } catch (reason) {
    error.value = `无法强制修复应用配置：${messageOf(reason)}`
  } finally {
    applicationResetBusy.value = false
  }
}

export async function copyApplicationConfigDirectory() {
  if (!applicationConfigDirectory.value) return
  try {
    await navigator.clipboard.writeText(applicationConfigDirectory.value)
    notice.value = '配置目录路径已复制。'
  } catch (reason) {
    error.value = `无法复制配置目录路径：${messageOf(reason)}`
  }
}
