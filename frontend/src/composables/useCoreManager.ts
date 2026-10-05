import { computed, ref } from 'vue'
import {
  ApplyCoreConfiguration,
  GetCoreStatus,
  GetRecoveryStatus,
  StopCore,
  ValidateCoreConfiguration,
} from '../../wailsjs/go/app/App'
import type { core } from '../../wailsjs/go/models'
import { t } from '../i18n'
import type { RecoveryStatus } from '../types'
import type { ApplicationStatus } from '../vite-env'
import { addLog } from './useAppLogs'
import { busy, error, messageOf, notice } from './useFeedback'
import { currentView, setView } from './useNavigation'
import {
  blockingDiagnostics,
  hasSavedSelection,
  setupRequired,
} from './useNetworkInterfaces'
import {
  buildConfig,
  flushRuleSettings,
  geoRulesNotReady,
} from './useRulesManager'

export const app = ref<ApplicationStatus>({
  name: 'WinRouter',
  version: 'development',
  commit: 'unknown',
  coreVersion: '1.13.15',
  ready: false,
})

export const coreStatus = ref<core.Status>({
  state: 'stopped',
  generation: 0,
  restart_attempts: 0,
  abnormal: false,
})

export const recoveryStatus = ref<RecoveryStatus>({
  state: 'idle',
  desired_running: false,
  attempts: 0,
})

export const isRunning = computed(() => coreStatus.value.state === 'running')
export const isRecovering = computed(() =>
  ['stopping', 'waiting-for-network', 'recovering'].includes(recoveryStatus.value.state)
)

export const stateLabel = computed(() => {
  if (busy.value) return t('state.processing')
  if (recoveryStatus.value.state === 'waiting-for-network') return t('state.waitingNetwork')
  if (recoveryStatus.value.state === 'stopping') return t('state.stopping')
  if (recoveryStatus.value.state === 'recovering') return t('state.recovering')
  if (recoveryStatus.value.state === 'failed') return t('state.recoveryFailed')
  if (coreStatus.value.abnormal) return t('state.needsRecovery')
  if (isRunning.value) return t('state.running')
  if (hasSavedSelection.value) return t('state.ready')
  return t('state.unconfigured')
})

export const pageTitle = computed(() => {
  if (currentView.value === 'overview') return t('title.overview')
  if (currentView.value === 'monitoring') return t('title.monitoring')
  if (currentView.value === 'interfaces') return t(setupRequired.value ? 'title.interfacesFirst' : 'title.interfaces')
  if (currentView.value === 'rules') return t('title.rules')
  if (currentView.value === 'proxy') return t('title.proxy')
  if (currentView.value === 'diagnostics') return t('title.diagnostics')
  return t('title.settings')
})

export async function refreshStatus() {
  try {
    const [core, recovery] = await Promise.all([GetCoreStatus(), GetRecoveryStatus()])
    coreStatus.value = core
    recoveryStatus.value = recovery as RecoveryStatus
  } catch (reason) {
    addLog('warning', `无法读取核心状态：${messageOf(reason)}`)
  }
}

export async function startCore() {
  error.value = ''
  notice.value = ''
  if (geoRulesNotReady.value.length) {
    error.value = `以下规则集尚无可用数据：${geoRulesNotReady.value.map(source => source.name).join('、')}。请先完成更新。`
    setView('rules')
    return
  }
  if (!hasSavedSelection.value || blockingDiagnostics.value.length) {
    error.value = '预检未通过。请先完成网卡选择并处理错误诊断。'
    setView('interfaces')
    return
  }
  busy.value = true
  try {
    await flushRuleSettings()
    const input = buildConfig()
    await ValidateCoreConfiguration(input)
    addLog('info', '配置预检通过，正在请求管理员权限。')
    coreStatus.value = await ApplyCoreConfiguration(input)
    notice.value = '分流核心已启动。'
    addLog('info', `核心已应用，进程 ${coreStatus.value.pid ?? '未知'}，代次 ${coreStatus.value.generation}`)
  } catch (reason) {
    error.value = `启动失败：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally {
    busy.value = false
  }
}

export async function stopCore() {
  error.value = ''
  notice.value = ''
  busy.value = true
  try {
    coreStatus.value = await StopCore()
    notice.value = '分流核心已停止，系统网络已恢复。'
    addLog('info', `核心已停止，代次 ${coreStatus.value.generation}`)
  } catch (reason) {
    error.value = `停止失败：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally {
    busy.value = false
  }
}
