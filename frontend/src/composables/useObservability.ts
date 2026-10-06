import { computed, ref } from 'vue'
import { GetConnectionObservationEnabled, GetObservations, SetConnectionObservationEnabled } from '../../wailsjs/go/app/App'
import { t } from '../i18n'
import type { ConnectionEvent, ConnectionSummary, CounterBaseline, observability, RuleHit, TrafficBudgetStatus, TrafficPoint, UsageBaseline } from '../types'
import { addLog } from './useAppLogs'
import { error, messageOf, notice } from './useFeedback'
import { selectedA, selectedB } from './useNetworkInterfaces'

export const emptyTrafficBudget: TrafficBudgetStatus = {
  enabled: false,
  budget_gb: 100,
  warning_percent: 80,
  period: '',
  used_bytes: 0,
  budget_bytes: 0,
  used_percent: 0,
  warning_reached: false,
  limit_reached: false,
}

export const observations = ref<{
  probes: observability.ProbeResult[]
  counters: observability.InterfaceCounter[]
  rule_sets: observability.RuleSetMetadata[]
  connections: ConnectionSummary
  rule_hits: RuleHit[]
  connection_observation: boolean
  connection_events: ConnectionEvent[]
  traffic_budget: TrafficBudgetStatus
}>({
  probes: [],
  counters: [],
  rule_sets: [],
  connections: { active_tcp: 0, established_tcp: 0, listening_tcp: 0, udp_endpoints: 0 },
  rule_hits: [],
  connection_observation: false,
  connection_events: [],
  traffic_budget: emptyTrafficBudget,
})

export const observationsError = ref('')
export const traffic = ref<TrafficPoint[]>([])
export const counterBaselines = new Map<string, CounterBaseline>()
export const usageBaselines = ref<Record<string, UsageBaseline>>({})

export const latestTraffic = computed(() => traffic.value.at(-1) ?? { at: 0, aDown: 0, aUp: 0, bDown: 0, bUp: 0 })
export const maxTrafficRateA = computed(() => Math.max(1, ...traffic.value.flatMap(point => [point.aDown, point.aUp])))
export const maxTrafficRateB = computed(() => Math.max(1, ...traffic.value.flatMap(point => [point.bDown, point.bUp])))
export const counterA = computed(() => observations.value.counters.find(item => item.guid.toLowerCase() === selectedA.value.toLowerCase()))
export const counterB = computed(() => observations.value.counters.find(item => item.guid.toLowerCase() === selectedB.value.toLowerCase()))
export const ruleHitA = computed(() => observations.value.rule_hits.find(item => item.outbound === 'a-direct')?.count ?? 0)
export const ruleHitB = computed(() => observations.value.rule_hits.find(item => item.outbound === 'b-direct')?.count ?? 0)

export function observationCount(value: unknown): number {
  const count = Number(value)
  return Number.isFinite(count) && count >= 0 ? Math.floor(count) : 0
}

export function normalizeObservations(value?: Partial<typeof observations.value>) {
  const rawConnections = (value?.connections ?? {}) as Partial<ConnectionSummary>
  const rawHits = Array.isArray(value?.rule_hits) ? value.rule_hits : []
  return {
    probes: value?.probes ?? [],
    counters: value?.counters ?? [],
    rule_sets: value?.rule_sets ?? [],
    connections: {
      active_tcp: observationCount(rawConnections.active_tcp),
      established_tcp: observationCount(rawConnections.established_tcp),
      listening_tcp: observationCount(rawConnections.listening_tcp),
      udp_endpoints: observationCount(rawConnections.udp_endpoints),
      sampled_at: rawConnections.sampled_at,
    },
    rule_hits: rawHits.map(item => ({ outbound: String(item?.outbound ?? ''), count: observationCount(item?.count) })),
    connection_observation: Boolean(value?.connection_observation),
    connection_events: Array.isArray(value?.connection_events) ? value.connection_events : [],
    traffic_budget: value?.traffic_budget ?? emptyTrafficBudget,
  }
}

export function sampleTraffic(counters: observability.InterfaceCounter[]) {
  const rates: Record<string, { down: number; up: number }> = {}
  for (const counter of counters) {
    const at = new Date(counter.sampled_at).getTime()
    const previous = counterBaselines.get(counter.guid)
    if (previous && at > previous.at) {
      const seconds = (at - previous.at) / 1000
      rates[counter.guid] = {
        down: counter.received_bytes >= previous.received ? (counter.received_bytes - previous.received) / seconds : 0,
        up: counter.transmitted_bytes >= previous.transmitted ? (counter.transmitted_bytes - previous.transmitted) / seconds : 0,
      }
    }
    counterBaselines.set(counter.guid, { at, received: counter.received_bytes, transmitted: counter.transmitted_bytes })
  }
  if (!rates[selectedA.value] && !rates[selectedB.value]) return
  traffic.value.push({
    at: Date.now(),
    aDown: rates[selectedA.value]?.down ?? 0,
    aUp: rates[selectedA.value]?.up ?? 0,
    bDown: rates[selectedB.value]?.down ?? 0,
    bUp: rates[selectedB.value]?.up ?? 0,
  })
  traffic.value = traffic.value.slice(-30)
}

export async function refreshObservations() {
  try {
    const next = normalizeObservations(await GetObservations())
    observations.value = next
    observationsError.value = ''
    sampleTraffic(next.counters)
  } catch (reason) {
    observationsError.value = messageOf(reason)
    addLog('warning', `无法读取诊断数据：${observationsError.value}`)
  }
}

export async function toggleConnectionObservation() {
  error.value = ''
  try {
    await SetConnectionObservationEnabled(!observations.value.connection_observation)
    observations.value.connection_observation = await GetConnectionObservationEnabled()
    notice.value = observations.value.connection_observation
      ? '连接观测已开启，下次启动核心后生效。'
      : '连接观测已关闭。'
  } catch (reason) {
    error.value = messageOf(reason)
  }
}

export function formatRate(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB/s`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB/s`
  return `${Math.round(bytes)} B/s`
}

export function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${Math.round(bytes / 1024)} KB`
}

export function displayedUsage(counter: observability.InterfaceCounter | undefined, direction: 'received' | 'transmitted') {
  if (!counter) return 0
  const baseline = usageBaselines.value[counter.guid.toLowerCase()]?.[direction] ?? 0
  const current = direction === 'received' ? counter.received_bytes : counter.transmitted_bytes
  return current >= baseline ? current - baseline : current
}

export function resetInterfaceUsage(role: 'A' | 'B') {
  const counter = role === 'A' ? counterA.value : counterB.value
  if (!counter) return
  usageBaselines.value = {
    ...usageBaselines.value,
    [counter.guid.toLowerCase()]: {
      received: counter.received_bytes,
      transmitted: counter.transmitted_bytes,
      reset_at: new Date().toISOString(),
    },
  }
  localStorage.setItem('winrouter.trafficBaselines.v1', JSON.stringify(usageBaselines.value))
  notice.value = `网卡 ${role} 的本地累计流量显示已清零。`
}

export function trafficHeight(value: number, role: 'A' | 'B') {
  const maximum = role === 'A' ? maxTrafficRateA.value : maxTrafficRateB.value
  return `${Math.max(2, Math.round((value / maximum) * 100))}%`
}
