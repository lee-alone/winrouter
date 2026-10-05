import { computed, ref } from 'vue'
import {
  ExportDiagnosticBundle,
  PreviewDiagnosticBundle,
  RunHealthProbe,
} from '../../wailsjs/go/app/App'
import type { observability } from '../../wailsjs/go/models'
import type { DiagnosticRecommendation } from '../types'
import { addLog } from './useAppLogs'
import { coreStatus, recoveryStatus } from './useCoreManager'
import { busy, error, messageOf, notice } from './useFeedback'
import { setView } from './useNavigation'
import {
  selectedAdapterA,
  selectedAdapterB,
  snapshot,
} from './useNetworkInterfaces'
import { observations, refreshObservations } from './useObservability'

export const probeProtocol = ref('tcp')
export const probeTarget = ref('1.1.1.1:443')
export const probeDNSName = ref('example.com')
export const probeRole = ref<'A' | 'B'>('B')
export const diagnosticPreview = ref<observability.BundlePreview>()

export const diagnosticRecommendations = computed<DiagnosticRecommendation[]>(() => {
  const recommendations: DiagnosticRecommendation[] = []
  for (const item of snapshot.value?.diagnostics ?? []) {
    if (item.code.startsWith('missing-default-route-')) {
      recommendations.push({
        severity: 'error',
        title: '出口缺少 IPv4 默认路由',
        detail: '请恢复该物理网卡的网关连接，或重新选择具有 IPv4 网关的出口。',
        action: 'interfaces',
      })
    } else if (item.code === 'prefix-overlap') {
      recommendations.push({
        severity: 'error',
        title: '局域网前缀发生重叠',
        detail: '两块出口的私有网段无法安全区分，请更换一块出口或调整网络地址后重新保存。',
        action: 'interfaces',
      })
    } else if (item.code === 'tun-pool-exhausted' || item.code === 'prefix-overlap-tun') {
      recommendations.push({
        severity: 'error',
        title: 'TUN 地址池不可用',
        detail: '请关闭冲突的虚拟网卡/TUN，确认地址池空闲后重新执行预检。',
        action: 'interfaces',
      })
    } else if (item.code === 'same-interface') {
      recommendations.push({
        severity: 'error',
        title: '出口 A 与出口 B 相同',
        detail: '请选择两块不同的物理网卡，避免流量无法按出口区分。',
        action: 'interfaces',
      })
    } else {
      recommendations.push({
        severity: item.severity === 'error' ? 'error' : 'warning',
        title: item.code,
        detail: item.message,
        action: 'interfaces',
      })
    }
  }

  if (recoveryStatus.value.state === 'waiting-for-network') {
    recommendations.push({
      severity: 'warning',
      title: '正在等待网络恢复',
      detail: '核心已安全停止。请等待物理网卡恢复，系统会在稳定窗口后自动重建。',
      action: 'none',
    })
  }

  if (recoveryStatus.value.state === 'failed' || coreStatus.value.abnormal || coreStatus.value.state === 'failed') {
    recommendations.push({
      severity: 'error',
      title: '核心最近一次运行失败',
      detail: recoveryStatus.value.last_error || coreStatus.value.last_error || '请导出脱敏诊断包以便定位失败原因。',
      action: 'export',
    })
  }

  const latestProbe = observations.value.probes.at(-1)
  if (latestProbe && !latestProbe.success) {
    recommendations.push({
      severity: 'warning',
      title: '最近一次主动探测失败',
      detail: latestProbe.error || '请检查目标地址、预期出口和当前网卡连接。',
      action: 'probe',
    })
  }

  if (!recommendations.length) {
    recommendations.push({
      severity: 'info',
      title: '未发现需要处理的问题',
      detail: '当前接口、核心状态和最近探测结果没有产生阻断诊断。',
      action: 'none',
    })
  }

  return recommendations
})

export async function runProbe() {
  error.value = ''
  busy.value = true
  try {
    const adapter = probeRole.value === 'A' ? selectedAdapterA.value : selectedAdapterB.value
    const result = await RunHealthProbe({
      protocol: probeProtocol.value,
      target: probeTarget.value,
      dns_name: probeProtocol.value === 'dns' ? probeDNSName.value : '',
      expected_interface: adapter?.guid ?? '',
      timeout_ms: 5000,
    })
    await refreshObservations()
    const expected = adapter ? `${adapter.friendly_name} (${adapter.guid})` : '未选择接口'
    const actualAdapter = snapshot.value?.adapters.find(
      item => item.guid.toLowerCase() === (result.actual_interface ?? '').toLowerCase()
    )
    const actual = actualAdapter
      ? `${actualAdapter.friendly_name} (${actualAdapter.guid})`
      : result.actual_interface || '未识别接口'
    const detail = `${probeProtocol.value.toUpperCase()} 探测${result.success ? '通过' : '失败'}：预期 ${expected}，实际 ${actual}，源地址 ${result.source_address || '未知'}，${result.duration_ms} ms${result.error ? `；${result.error}` : ''}`
    if (!result.success) error.value = detail
    addLog(result.success ? 'info' : 'warning', detail)
  } catch (reason) {
    error.value = `探测失败：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally {
    busy.value = false
  }
}

export async function previewDiagnostics() {
  try {
    diagnosticPreview.value = await PreviewDiagnosticBundle()
  } catch (reason) {
    error.value = `无法生成诊断预览：${messageOf(reason)}`
  }
}

export async function exportDiagnostics() {
  busy.value = true
  try {
    const path = await ExportDiagnosticBundle()
    if (path) {
      notice.value = `诊断包已导出：${path}`
      addLog('info', '诊断包已完成脱敏并导出。')
    }
  } catch (reason) {
    error.value = `导出失败：${messageOf(reason)}`
  } finally {
    busy.value = false
  }
}

export async function applyRecommendation(action: DiagnosticRecommendation['action']) {
  if (action === 'interfaces') setView('interfaces')
  if (action === 'probe') document.querySelector<HTMLInputElement>('.probe-target input')?.focus()
  if (action === 'export') await previewDiagnostics()
}

export function recommendationActionText(action: DiagnosticRecommendation['action']) {
  return { interfaces: '检查网卡', probe: '重新探测', export: '准备诊断包', none: '' }[action]
}
