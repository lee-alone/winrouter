import { computed, ref, watch } from 'vue'
import { SelectInterfaces, SelectSingleInterface, SetIPv6Policy, SetRoutingMode } from '../../wailsjs/go/app/App'
import type { interfacemanager, interfaces } from '../types'
import { addLog } from './useAppLogs'
import { busy, error, messageOf, notice } from './useFeedback'
import { setView } from './useNavigation'

export const snapshot = ref<interfacemanager.Snapshot>()
export const routingMode = ref<'single' | 'dual'>('single')
export const selectedA = ref('')
export const selectedB = ref('')
export const sameAdapterWarning = ref(false)
export const ipv6Policy = ref<'block' | 'split'>('block')
export const ipv6Busy = ref(false)

export function adapterByGUID(guid: string): interfaces.Adapter | undefined {
  return snapshot.value?.adapters.find(adapter => adapter.guid === guid)
}

export const candidates = computed(() => snapshot.value?.candidates ?? [])
export const selectedAdapterA = computed(() => adapterByGUID(selectedA.value))
export const selectedAdapterB = computed(() => adapterByGUID(selectedB.value))

export const hasSavedSelection = computed(() => {
  if (routingMode.value === 'single') {
    return snapshot.value?.interface_a.status === 'resolved'
  }
  return snapshot.value?.interface_a.status === 'resolved' && snapshot.value?.interface_b.status === 'resolved'
})

export const setupRequired = computed(() => !hasSavedSelection.value)

export const directPrefixes = computed(() =>
  (snapshot.value?.topology.prefixes ?? []).filter(
    prefix =>
      prefix.action === 'bind-interface' &&
      (ipv6Policy.value === 'split' || !prefix.prefix.includes(':')) &&
      !prefix.prefix.startsWith('fe80:') &&
      (prefix.adapter_guid === selectedA.value || (routingMode.value === 'dual' && prefix.adapter_guid === selectedB.value))
  )
)

export const blockingDiagnostics = computed(() =>
  (snapshot.value?.diagnostics ?? []).filter(item => item.severity === 'error')
)

watch([selectedA, selectedB], ([newA, newB]) => {
  if (routingMode.value === 'single') {
    sameAdapterWarning.value = false
    if (error.value.includes('不能绑定相同网卡')) {
      error.value = ''
    }
    return
  }
  if (sameAdapterWarning.value && newA && newB && newA !== newB) {
    sameAdapterWarning.value = false
    if (error.value.includes('不能绑定相同网卡')) {
      error.value = ''
    }
  }
})

export function syncSelection(next: interfacemanager.Snapshot) {
  snapshot.value = next
  if (next.mode === 'dual' || next.mode === 'single') {
    routingMode.value = next.mode
  } else if (!next.interface_b?.saved?.guid && candidates.value.length < 2) {
    routingMode.value = 'single'
  }
  if (!selectedA.value && next.interface_a.match) selectedA.value = next.interface_a.match.adapter.guid
  if (!selectedB.value && next.interface_b.match) selectedB.value = next.interface_b.match.adapter.guid
  if (!selectedA.value && next.candidates[0]) selectedA.value = next.candidates[0].adapter.guid
  if (routingMode.value === 'dual') {
    if (!selectedB.value) selectedB.value = next.candidates.find(candidate => candidate.adapter.guid !== selectedA.value)?.adapter.guid ?? ''
  }
}

export async function switchRoutingMode(mode: 'single' | 'dual', isRunning = false) {
  if (routingMode.value === mode) return
  if (isRunning) {
    window.alert('更改分流模式前，请先停止分流核心。')
    return
  }
  routingMode.value = mode
  sameAdapterWarning.value = false
  if (error.value.includes('不能绑定相同网卡') || error.value.includes('前缀重叠') || error.value.includes('冲突')) {
    error.value = ''
  }
  try {
    const next = await SetRoutingMode(mode)
    syncSelection(next)
    addLog('info', `已切换分流模式：${mode === 'single' ? '单网卡代理模式' : '双网卡物理分流模式'}`)
  } catch (err) {
    addLog('warning', `切换模式预检：${messageOf(err)}`)
  }
}

export async function saveSelection() {
  error.value = ''
  notice.value = ''
  if (routingMode.value === 'single') {
    if (!selectedA.value || !selectedAdapterA.value) {
      const msg = '请选择出网物理网卡。'
      error.value = msg
      window.alert(`警告：${msg}`)
      return
    }
    sameAdapterWarning.value = false
    const changing = hasSavedSelection.value && snapshot.value?.interface_a.match?.adapter.guid !== selectedA.value
    if (changing && !window.confirm('更改网卡会使当前策略失效。确认保存新的出网网卡吗？')) return
    busy.value = true
    try {
      const next = await SelectSingleInterface(selectedA.value)
      syncSelection(next)
      notice.value = '单网卡选择已保存，预检信息已更新。'
      addLog('info', `已选择出网网卡：${selectedAdapterA.value?.friendly_name}`)
      setView('overview')
    } catch (reason) {
      error.value = `无法保存网卡：${messageOf(reason)}`
      addLog('error', error.value, true)
    } finally {
      busy.value = false
    }
    return
  }

  if (!selectedA.value || !selectedB.value || !selectedAdapterA.value || !selectedAdapterB.value) {
    const msg = '请为出口 A 和出口 B 分别选择可用的物理网卡。'
    error.value = msg
    window.alert(`警告：${msg}`)
    return
  }
  if (selectedA.value === selectedB.value) {
    sameAdapterWarning.value = true
    const warnMsg = '出口 A 和出口 B 不能绑定相同网卡，请为两个出口分别选择不同的物理网卡。'
    error.value = warnMsg
    addLog('warning', `保存网卡失败：出口 A 和出口 B 绑定了相同网卡 (${selectedAdapterA.value?.friendly_name || selectedA.value})`)
    window.alert(`警告：出口 A 和出口 B 不能绑定相同网卡！\n\n请为两个出口分别选择不同的物理网卡。`)
    return
  }
  sameAdapterWarning.value = false
  const changing =
    hasSavedSelection.value &&
    (snapshot.value?.interface_a.match?.adapter.guid !== selectedA.value ||
      snapshot.value?.interface_b.match?.adapter.guid !== selectedB.value)
  if (changing && !window.confirm('更改网卡会使当前策略失效。确认保存新的出口网卡吗？')) return
  busy.value = true
  try {
    const next = await SelectInterfaces(selectedA.value, selectedB.value)
    syncSelection(next)
    notice.value = '网卡选择已保存，预检信息已更新。'
    addLog('info', `已选择出口 A：${selectedAdapterA.value?.friendly_name}；出口 B：${selectedAdapterB.value?.friendly_name}`)
    setView('overview')
  } catch (reason) {
    error.value = `无法保存网卡：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally {
    busy.value = false
  }
}

export async function saveIPv6Policy(onSaved?: () => Promise<void> | void) {
  ipv6Busy.value = true
  error.value = ''
  notice.value = ''
  try {
    const next = await SetIPv6Policy(ipv6Policy.value)
    syncSelection(next)
    notice.value =
      ipv6Policy.value === 'split'
        ? next.diagnostics.some(item => item.severity === 'error')
          ? 'IPv6 分流设置已保存，但当前网络未通过 IPv6 预检。'
          : 'IPv6 分流已启用并完成预检。'
        : 'IPv6 分流已关闭，IPv6 流量将被阻止。'
    if (onSaved) await onSaved()
  } catch (reason) {
    error.value = `无法更新 IPv6 策略：${messageOf(reason)}`
  } finally {
    ipv6Busy.value = false
  }
}
