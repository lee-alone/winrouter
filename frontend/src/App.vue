<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { AddProxyNode, AddSubscription, ApplyCoreConfiguration, ApplySelectedProxyConfiguration, ConfigureSRSSource, DeleteProxyNode, DeleteSRSSource, DeleteSubscription, ExportDiagnosticBundle, GetApplicationConfigDirectory, GetAutostartStatus, GetConnectionObservationEnabled, GetCoreStatus, GetDNSPresets, GetDNSSettings, GetInterfaceSnapshot, GetIPv6Policy, GetObservations, GetRecoveryStatus, GetRuleSettings, GetSRSPresets, GetStatus, GetTrafficBudgetStatus, InspectProcessRules, ListProxyNodes, ListSRSSources, ListSubscriptions, PreviewCoreRules, PreviewDiagnosticBundle, RecordApplicationLog, RefreshSRSSource, RefreshSubscription, RepairApplicationSettings, ResetApplicationSettings, ResetInterfaceSelection, ResetTrafficBudget, ResetWindowsNetworkStack, RunHealthProbe, SelectInterfaces, SelectProxyNode, SetAutostartEnabled, SetConnectionObservationEnabled, SetDNSSettings, SetIPv6Policy, SetProxyNodeFavorite, SetRuleSettings, SetTrafficBudget, SpeedTestProxyNodes, StopCore, TestDNSServer, TestProxyNode, UpdateProxyNode, UpdateSubscription, ValidateCoreConfiguration, ValidateSelectedProxyConfiguration } from '../wailsjs/go/main/App'
import type { config, core, interfacemanager, interfaces, main, nodes, observability, processrules, rulesettings, srssets, subscriptions } from '../wailsjs/go/models'
import { EventsOn } from '../wailsjs/runtime/runtime'
import type { ApplicationStatus } from './vite-env'
import { locale, setLocale, t, type Locale } from './i18n'

type View = 'overview' | 'monitoring' | 'interfaces' | 'rules' | 'proxy' | 'diagnostics' | 'settings'
type LogLevel = 'info' | 'warning' | 'error'
type LogEntry = { id: number; time: string; level: LogLevel; message: string; correlation?: string }
type RecoveryStatus = { state: 'idle' | 'monitoring' | 'stopping' | 'waiting-for-network' | 'recovering' | 'failed'; desired_running: boolean; attempts: number; last_error?: string; last_change?: string; snapshot_sequence?: number }

const app = ref<ApplicationStatus>({ name: 'WinRouter', version: 'development', commit: 'unknown', mode: 'direct-split', coreVersion: '1.13.15', ready: false })
const snapshot = ref<interfacemanager.Snapshot>()
const coreStatus = ref<core.Status>({ state: 'stopped', generation: 0, restart_attempts: 0, abnormal: false })
const recoveryStatus = ref<RecoveryStatus>({ state: 'idle', desired_running: false, attempts: 0 })
const view = ref<View>('overview')
const selectedA = ref('')
const selectedB = ref('')
const busy = ref(false)
const notice = ref('')
const error = ref('')
const logFilter = ref<'all' | LogLevel>('all')
const logs = ref<LogEntry[]>([])
type ConnectionSummary = { active_tcp: number; established_tcp: number; listening_tcp: number; udp_endpoints: number; sampled_at?: string }
type RuleHit = { outbound: string; count: number }
type ConnectionEvent = { domain?: string; address_type?: string; ip?: string; protocol?: string; rule?: string; outbound?: string; attempts: number; established: number; bytes_up: number; bytes_down: number; last_seen?: string }
type TrafficPoint = { at: number; aDown: number; aUp: number; bDown: number; bUp: number }
type CounterBaseline = { at: number; received: number; transmitted: number }
type UsageBaseline = { received: number; transmitted: number; reset_at: string }
type DiagnosticRecommendation = { severity: 'error' | 'warning' | 'info'; title: string; detail: string; action: 'interfaces' | 'probe' | 'export' | 'none' }
type TrafficBudgetStatus = { enabled: boolean; budget_gb: number; warning_percent: number; period: string; used_bytes: number; budget_bytes: number; used_percent: number; warning_reached: boolean; limit_reached: boolean; interface_guid?: string }
type DNSServer = { preset_id?: string; type: 'udp' | 'tls' | 'https'; server: string; port: number; server_name?: string }
type DNSSettings = { schema_version: number; domestic: DNSServer; global: DNSServer }
type DNSPreset = DNSServer & { id: string; name: string; scope: 'domestic' | 'global' }
type DNSTestState = 'idle' | 'testing' | 'success' | 'failed'
function normalizeDNSSettings(value: DNSSettings): DNSSettings {
  const normalizeServer = (server: DNSServer): DNSServer => ({ ...server, preset_id: server.preset_id ?? '', server_name: server.server_name ?? '' })
  return { ...value, domestic: normalizeServer(value.domestic), global: normalizeServer(value.global) }
}
const emptyTrafficBudget: TrafficBudgetStatus = { enabled: false, budget_gb: 100, warning_percent: 80, period: '', used_bytes: 0, budget_bytes: 0, used_percent: 0, warning_reached: false, limit_reached: false }
const observations = ref<{ probes: observability.ProbeResult[]; counters: observability.InterfaceCounter[]; rule_sets: observability.RuleSetMetadata[]; connections: ConnectionSummary; rule_hits: RuleHit[]; connection_observation: boolean; connection_events: ConnectionEvent[]; traffic_budget: TrafficBudgetStatus }>({ probes: [], counters: [], rule_sets: [], connections: { active_tcp: 0, established_tcp: 0, listening_tcp: 0, udp_endpoints: 0 }, rule_hits: [], connection_observation: false, connection_events: [], traffic_budget: emptyTrafficBudget })
const traffic = ref<TrafficPoint[]>([])
const counterBaselines = new Map<string, CounterBaseline>()
const usageBaselines = ref<Record<string, UsageBaseline>>({})
const probeProtocol = ref('tcp')
const probeTarget = ref('1.1.1.1:443')
const probeDNSName = ref('example.com')
const probeRole = ref<'A' | 'B'>('B')
const diagnosticPreview = ref<observability.BundlePreview>()
const autostartEnabled = ref(false)
const autostartBusy = ref(false)
const budgetBusy = ref(false)
const budgetForm = ref({ enabled: false, budget_gb: 100, warning_percent: 80 })
const dnsSettings = ref<DNSSettings>({ schema_version: 1, domestic: { preset_id: 'aliyun-udp', type: 'udp', server: '223.5.5.5', port: 53 }, global: { preset_id: 'google-udp', type: 'udp', server: '8.8.8.8', port: 53 } })
const dnsPresets = ref<DNSPreset[]>([])
const dnsBusy = ref(false)
const dnsTests = ref<Record<'domestic' | 'global', DNSTestState>>({ domestic: 'idle', global: 'idle' })
const observationsError = ref('')
const ipv6Policy = ref<'block' | 'split'>('block')
const ipv6Busy = ref(false)
const networkResetBusy = ref(false)
const networkResetResult = ref<main.NetworkResetResult>()
const applicationResetBusy = ref(false)
const initializationFailed = ref(false)
const applicationConfigDirectory = ref('')
interface ProxyProtocolMeta {
  value: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  label: string
  authKind: 'user-pass' | 'ss' | 'uuid' | 'trojan'
  supportsTLS: boolean
  forceTLS: boolean
  supportsTransport: boolean
  supportsFlow: boolean
  allowNoSecret: boolean
}

const proxyProtocols: ProxyProtocolMeta[] = [
  { value: 'http', label: 'HTTP CONNECT', authKind: 'user-pass', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: true },
  { value: 'shadowsocks', label: 'Shadowsocks', authKind: 'ss', supportsTLS: false, forceTLS: false, supportsTransport: false, supportsFlow: false, allowNoSecret: false },
  { value: 'vmess', label: 'VMess', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
  { value: 'vless', label: 'VLESS', authKind: 'uuid', supportsTLS: true, forceTLS: false, supportsTransport: true, supportsFlow: true, allowNoSecret: false },
  { value: 'trojan', label: 'Trojan', authKind: 'trojan', supportsTLS: true, forceTLS: true, supportsTransport: true, supportsFlow: false, allowNoSecret: false },
]

const ssMethods = [
  'aes-128-gcm',
  'aes-192-gcm',
  'aes-256-gcm',
  'chacha20-ietf-poly1305',
  'xchacha20-ietf-poly1305',
]

const proxyNodes = ref<nodes.Node[]>([])
const proxyFormOpen = ref(false)
const proxyForm = ref({
  id: '',
  original_type: 'http' as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan',
  original_has_secret: false,
  name: '',
  type: 'http' as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan',
  server: '',
  port: 8080,
  egress: 'b' as 'a' | 'b',
  username: '',
  password: '',
  method: 'aes-256-gcm',
  uuid: '',
  flow: '',
  tls_enabled: false,
  tls_server_name: '',
  tls_insecure: false,
  tls_alpn: '',
  transport_type: 'tcp' as 'tcp' | 'ws',
  transport_path: '',
  transport_host: '',
  has_saved_secret: false,
  replace_secret: false,
  clear_secret: false,
})
const proxyFieldErrors = ref<Record<string, string>>({})
const proxyImportOpen = ref(false)
const proxyImportURI = ref('')
const proxyImportError = ref('')
const selectedMode = ref<'direct-split' | 'proxy-split'>('direct-split')
const defaultOutbound = ref<'a' | 'b'>('b')
const proxyTests = ref<Record<string, nodes.TestResult>>({})
const testingProxyID = ref('')
const testingAllProxies = ref(false)
const proxySort = ref<'favorite' | 'latency' | 'name'>('favorite')
const subscriptionList = ref<subscriptions.Subscription[]>([])
const subscriptionFormOpen = ref(false)
const subscriptionForm = ref({ id: '', name: '', url: '' })
const refreshingSubscriptionID = ref('')
type InlineRuleType = 'domain-suffix' | 'domain' | 'ip' | 'process-name' | 'process-path'
type RuleAction = 'a' | 'b' | 'final' | 'reject'
type CustomRule = { id: string; name: string; type: InlineRuleType; values: string[]; action: RuleAction; enabled: boolean }
type RuleForm = { id: string; name: string; type: InlineRuleType | 'rule-set'; valuesText: string; action: RuleAction; enabled: boolean }
const customRules = ref<CustomRule[]>([])
const ruleOrder = ref<string[]>([])
const ruleFormOpen = ref(false)
const ruleForm = ref<RuleForm>({ id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true })
const rulePreview = ref<config.RulePreview[]>([])
const rulePreviewError = ref('')
const processRuleStatuses = ref<processrules.Status[]>([])
const srsPresets = ref<srssets.Preset[]>([])
const srsSources = ref<srssets.Source[]>([])
const srsBusyID = ref('')
const srsForm = ref({ id: '', name: '', kind: 'domain', preset_id: '', url: '', expected_sha256: '', enabled: true, action: 'a' })
let nextLogID = 1
let stopEvents: (() => void) | undefined
let stopRecoveryEvents: (() => void) | undefined
let stopTrayStartEvents: (() => void) | undefined
let stopBudgetEvents: (() => void) | undefined
let statusTimer: number | undefined
let ruleSettingsSave = Promise.resolve()
let ruleSettingsSaveError: unknown

const candidates = computed(() => snapshot.value?.candidates ?? [])
const selectedAdapterA = computed(() => adapterByGUID(selectedA.value))
const selectedAdapterB = computed(() => adapterByGUID(selectedB.value))
const geoRulesNotReady = computed(() => srsSources.value.filter(source => source.enabled && !source.applied_sha256))
type MasterRow = { key: string; kind: 'custom'; rule: CustomRule } | { key: string; kind: 'srs'; source: srssets.Source }
const masterRows = computed<MasterRow[]>(() => {
  const rows: MasterRow[] = []
  const seen = new Set<string>()
  for (const key of ruleOrder.value) {
    const rule = customRules.value.find(item => item.id === key)
    if (rule) { rows.push({ key, kind: 'custom', rule }); seen.add(key); continue }
    const source = srsSources.value.find(item => `srs:${item.id}` === key)
    if (source) { rows.push({ key, kind: 'srs', source }); seen.add(key) }
  }
  for (const rule of customRules.value) if (!seen.has(rule.id)) rows.push({ key: rule.id, kind: 'custom', rule })
  for (const source of srsSources.value) if (!seen.has(`srs:${source.id}`)) rows.push({ key: `srs:${source.id}`, kind: 'srs', source })
  return rows
})
const hasSavedSelection = computed(() => snapshot.value?.interface_a.status === 'resolved' && snapshot.value?.interface_b.status === 'resolved')
const selectionValid = computed(() => Boolean(selectedA.value && selectedB.value && selectedA.value !== selectedB.value && selectedAdapterA.value && selectedAdapterB.value))
const isRunning = computed(() => coreStatus.value.state === 'running')
const isRecovering = computed(() => ['stopping', 'waiting-for-network', 'recovering'].includes(recoveryStatus.value.state))
const filteredLogs = computed(() => logFilter.value === 'all' ? logs.value : logs.value.filter(entry => entry.level === logFilter.value))
const directPrefixes = computed(() => (snapshot.value?.topology.prefixes ?? []).filter(prefix => prefix.action === 'bind-interface' && (ipv6Policy.value === 'split' || !prefix.prefix.includes(':')) && !prefix.prefix.startsWith('fe80:') && (prefix.adapter_guid === selectedA.value || prefix.adapter_guid === selectedB.value)))
const blockingDiagnostics = computed(() => (snapshot.value?.diagnostics ?? []).filter(item => item.severity === 'error'))
const selectedProxyNode = computed(() => proxyNodes.value.find(node => node.selected))
const sortedProxyNodes = computed(() => [...proxyNodes.value].sort((first, second) => {
  if (proxySort.value === 'favorite' && first.favorite !== second.favorite) return first.favorite ? -1 : 1
  if (proxySort.value === 'latency') {
    const aTest = proxyTests.value[first.id]
    const bTest = proxyTests.value[second.id]
    const a = aTest?.available ? (aTest.total_ms || aTest.latency_ms) : Number.MAX_SAFE_INTEGER
    const b = bTest?.available ? (bTest.total_ms || bTest.latency_ms) : Number.MAX_SAFE_INTEGER
    if (a !== b) return a - b
  }
  return first.name.localeCompare(second.name, 'zh-CN')
}))

function formatErrorCategory(cat?: string) {
  switch (cat) {
    case 'dns_failed': return 'DNS 解析失败'
    case 'egress_unavailable': return '出口网卡不可用'
    case 'tcp_failed': return 'TCP 连接失败'
    case 'tls_failed': return 'TLS 握手失败'
    case 'authentication_failed': return '凭据认证失败'
    case 'protocol_failed': return '协议通信失败'
    case 'target_failed': return '测试目标异常'
    case 'timeout': return '请求超时'
    case 'core_failed': return '临时核心异常'
    default: return '测试失败'
  }
}
const proxyRiskMessages = computed(() => {
  const result: string[] = []
  const node = selectedProxyNode.value
  if (!node) result.push('未选择代理节点，代理模式无法启动。')
  else {
    if (node.server.includes('.') && !/^\d+\.\d+\.\d+\.\d+$/.test(node.server) && !node.resolved_ip) result.push('代理域名尚未通过出口 B DNS 解析和健康检查。')
    const tested = proxyTests.value[node.id]
    if (tested && !tested.available) result.push('所选代理最近一次可用性测试失败，启动将严格失败且不会直连回退。')
  }
  if (subscriptionList.value.some(item => item.last_error)) result.push('最近一次订阅更新失败，应用已保留上一批有效节点。')
  return result
})
const diagnosticRecommendations = computed<DiagnosticRecommendation[]>(() => {
  const recommendations: DiagnosticRecommendation[] = []
  for (const item of snapshot.value?.diagnostics ?? []) {
    if (item.code.startsWith('missing-default-route-')) recommendations.push({ severity: 'error', title: '出口缺少 IPv4 默认路由', detail: '请恢复该物理网卡的网关连接，或重新选择具有 IPv4 网关的出口。', action: 'interfaces' })
    else if (item.code === 'prefix-overlap') recommendations.push({ severity: 'error', title: '局域网前缀发生重叠', detail: '两块出口的私有网段无法安全区分，请更换一块出口或调整网络地址后重新保存。', action: 'interfaces' })
    else if (item.code === 'tun-pool-exhausted' || item.code === 'prefix-overlap-tun') recommendations.push({ severity: 'error', title: 'TUN 地址池不可用', detail: '请关闭冲突的虚拟网卡/TUN，确认地址池空闲后重新执行预检。', action: 'interfaces' })
    else if (item.code === 'same-interface') recommendations.push({ severity: 'error', title: '出口 A 与出口 B 相同', detail: '请选择两块不同的物理网卡，避免流量无法按出口区分。', action: 'interfaces' })
    else recommendations.push({ severity: item.severity === 'error' ? 'error' : 'warning', title: item.code, detail: item.message, action: 'interfaces' })
  }
  if (recoveryStatus.value.state === 'waiting-for-network') recommendations.push({ severity: 'warning', title: '正在等待网络恢复', detail: '核心已安全停止。请等待物理网卡恢复，系统会在稳定窗口后自动重建。', action: 'none' })
  if (recoveryStatus.value.state === 'failed' || coreStatus.value.abnormal || coreStatus.value.state === 'failed') recommendations.push({ severity: 'error', title: '核心最近一次运行失败', detail: recoveryStatus.value.last_error || coreStatus.value.last_error || '请导出脱敏诊断包以便定位失败原因。', action: 'export' })
  const latestProbe = observations.value.probes.at(-1)
  if (latestProbe && !latestProbe.success) recommendations.push({ severity: 'warning', title: '最近一次主动探测失败', detail: latestProbe.error || '请检查目标地址、预期出口和当前网卡连接。', action: 'probe' })
  if (!recommendations.length) recommendations.push({ severity: 'info', title: '未发现需要处理的问题', detail: '当前接口、核心状态和最近探测结果没有产生阻断诊断。', action: 'none' })
  return recommendations
})
const setupRequired = computed(() => !hasSavedSelection.value)
const latestTraffic = computed(() => traffic.value.at(-1) ?? { at: 0, aDown: 0, aUp: 0, bDown: 0, bUp: 0 })
const maxTrafficRateA = computed(() => Math.max(1, ...traffic.value.flatMap(point => [point.aDown, point.aUp])))
const maxTrafficRateB = computed(() => Math.max(1, ...traffic.value.flatMap(point => [point.bDown, point.bUp])))
const counterA = computed(() => observations.value.counters.find(item => item.guid.toLowerCase() === selectedA.value.toLowerCase()))
const counterB = computed(() => observations.value.counters.find(item => item.guid.toLowerCase() === selectedB.value.toLowerCase()))
const ruleHitA = computed(() => observations.value.rule_hits.find(item => item.outbound === 'a-direct')?.count ?? 0)
const ruleHitB = computed(() => observations.value.rule_hits.find(item => item.outbound === 'b-direct')?.count ?? 0)
const pageTitle = computed(() => view.value === 'overview' ? t('title.overview') : view.value === 'monitoring' ? t('title.monitoring') : view.value === 'interfaces' ? t(setupRequired.value ? 'title.interfacesFirst' : 'title.interfaces') : view.value === 'rules' ? t('title.rules') : view.value === 'proxy' ? t('title.proxy') : view.value === 'diagnostics' ? t('title.diagnostics') : t('title.settings'))
const stateLabel = computed(() => {
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

function adapterByGUID(guid: string): interfaces.Adapter | undefined {
  return snapshot.value?.adapters.find(adapter => adapter.guid === guid)
}

function addLog(level: LogLevel, message: string, correlated = false) {
  const correlation = correlated ? `WR-${Date.now().toString(36).toUpperCase()}-${nextLogID}` : undefined
  logs.value.unshift({ id: nextLogID++, time: new Date().toLocaleTimeString('zh-CN', { hour12: false }), level, message, correlation })
  logs.value = logs.value.slice(0, 200)
  void RecordApplicationLog(level, message, correlation ?? '')
}

function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
}

function formatSRSUpdatedAt(source: srssets.Source): string {
  if (!source.updated_at) return '尚未成功更新'
  const date = new Date(source.updated_at)
  if (Number.isNaN(date.getTime())) return '更新时间未知'
  return `最近更新：${date.toLocaleString('zh-CN', { hour12: false })}`
}

function resetSRSForm() {
  srsForm.value = { id: '', name: '', kind: 'domain', preset_id: '', url: '', expected_sha256: '', enabled: true, action: 'a' }
}

function chooseSRSPreset() {
  const preset = srsPresets.value.find(item => item.id === srsForm.value.preset_id)
  if (!preset) return
  srsForm.value.name = preset.name
  ruleForm.value.name = preset.name
  srsForm.value.kind = preset.kind
  srsForm.value.url = preset.url
  srsForm.value.expected_sha256 = ''
}

function editSRSSource(source: srssets.Source) {
  srsForm.value = { id: source.id, name: source.name, kind: source.kind, preset_id: source.preset_id || '', url: source.url, expected_sha256: source.expected_sha256 || '', enabled: source.enabled, action: source.action }
  ruleForm.value = { id: source.id, name: source.name, type: 'rule-set', valuesText: '', action: source.action as RuleAction, enabled: source.enabled }
  ruleFormOpen.value = true
}

async function submitSRSSource() {
  srsBusyID.value = srsForm.value.id || 'new'; error.value = ''; notice.value = ''
  try {
    const configured = await ConfigureSRSSource({ ...srsForm.value, name: ruleForm.value.name, action: ruleForm.value.action, enabled: ruleForm.value.enabled, applied_sha256: '', size: 0, last_error: '', upstream: '', license: '' } as srssets.Source)
    const key = `srs:${configured.id}`
    if (!ruleOrder.value.includes(key)) ruleOrder.value.push(key)
    srsSources.value = await ListSRSSources()
    await saveRuleSettings()
    notice.value = '规则集已保存。下载并验证成功前不会参与分流。'
    resetRuleForm()
    await refreshRulePreview()
  } catch (reason) { error.value = `无法保存 SRS 来源：${messageOf(reason)}` }
  finally { srsBusyID.value = '' }
}

async function refreshSRSSource(source: srssets.Source) {
  srsBusyID.value = source.id; error.value = ''; notice.value = ''
  try { await RefreshSRSSource(source.id); srsSources.value = await ListSRSSources(); notice.value = `${source.name} 已下载、校验并缓存。`; await refreshRulePreview() }
  catch (reason) { srsSources.value = await ListSRSSources(); error.value = `SRS 更新失败，已保留最后有效版本：${messageOf(reason)}` }
  finally { srsBusyID.value = '' }
}

async function toggleSRSSource(source: srssets.Source) {
  srsBusyID.value = source.id; error.value = ''; notice.value = ''
  try {
    await ConfigureSRSSource({ ...source, enabled: !source.enabled } as srssets.Source)
    srsSources.value = await ListSRSSources()
    notice.value = `${source.name} 已${source.enabled ? '停用' : '启用'}。`
    await refreshRulePreview()
  } catch (reason) { error.value = `无法更新规则状态：${messageOf(reason)}` }
  finally { srsBusyID.value = '' }
}

async function deleteSRSSource(source: srssets.Source) {
  if (!window.confirm(`删除 SRS 来源“${source.name}”？`)) return
  try { await DeleteSRSSource(source.id); srsSources.value = await ListSRSSources(); ruleOrder.value = ruleOrder.value.filter(key => key !== `srs:${source.id}`); await saveRuleSettings(); await refreshRulePreview(); notice.value = '自定义规则集已删除。' }
  catch (reason) { error.value = `无法删除 SRS 来源：${messageOf(reason)}` }
}

function observationCount(value: unknown): number {
  const count = Number(value)
  return Number.isFinite(count) && count >= 0 ? Math.floor(count) : 0
}

function normalizeObservations(value?: Partial<typeof observations.value>) {
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

async function toggleConnectionObservation() {
  error.value = ''
  try { await SetConnectionObservationEnabled(!observations.value.connection_observation); observations.value.connection_observation = await GetConnectionObservationEnabled(); notice.value = observations.value.connection_observation ? '连接观测已开启，下次启动核心后生效。' : '连接观测已关闭。' }
  catch (reason) { error.value = messageOf(reason) }
}

function sampleTraffic(counters: observability.InterfaceCounter[]) {
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
  traffic.value.push({ at: Date.now(), aDown: rates[selectedA.value]?.down ?? 0, aUp: rates[selectedA.value]?.up ?? 0, bDown: rates[selectedB.value]?.down ?? 0, bUp: rates[selectedB.value]?.up ?? 0 })
  traffic.value = traffic.value.slice(-30)
}

function syncSelection(next: interfacemanager.Snapshot) {
  snapshot.value = next
  if (!selectedA.value && next.interface_a.match) selectedA.value = next.interface_a.match.adapter.guid
  if (!selectedB.value && next.interface_b.match) selectedB.value = next.interface_b.match.adapter.guid
  if (!selectedA.value && next.candidates[0]) selectedA.value = next.candidates[0].adapter.guid
  if (!selectedB.value) selectedB.value = next.candidates.find(candidate => candidate.adapter.guid !== selectedA.value)?.adapter.guid ?? ''
}

async function refreshStatus() {
  try {
    const [core, recovery] = await Promise.all([GetCoreStatus(), GetRecoveryStatus()])
    coreStatus.value = core
    recoveryStatus.value = recovery as RecoveryStatus
  } catch (reason) { addLog('warning', `无法读取核心状态：${messageOf(reason)}`) }
}

async function refreshObservations() {
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

async function runProbe() {
  error.value = ''; busy.value = true
  try {
    const adapter = probeRole.value === 'A' ? selectedAdapterA.value : selectedAdapterB.value
    const result = await RunHealthProbe({ protocol: probeProtocol.value, target: probeTarget.value, dns_name: probeProtocol.value === 'dns' ? probeDNSName.value : '', expected_interface: adapter?.guid ?? '', timeout_ms: 5000 })
    await refreshObservations()
    const expected = adapter ? `${adapter.friendly_name} (${adapter.guid})` : '未选择接口'
    const actualAdapter = snapshot.value?.adapters.find(item => item.guid.toLowerCase() === (result.actual_interface ?? '').toLowerCase())
    const actual = actualAdapter ? `${actualAdapter.friendly_name} (${actualAdapter.guid})` : (result.actual_interface || '未识别接口')
    const detail = `${probeProtocol.value.toUpperCase()} 探测${result.success ? '通过' : '失败'}：预期 ${expected}，实际 ${actual}，源地址 ${result.source_address || '未知'}，${result.duration_ms} ms${result.error ? `；${result.error}` : ''}`
    if (!result.success) error.value = detail
    addLog(result.success ? 'info' : 'warning', detail)
  } catch (reason) { error.value = `探测失败：${messageOf(reason)}`; addLog('error', error.value, true) } finally { busy.value = false }
}

async function previewDiagnostics() {
  try { diagnosticPreview.value = await PreviewDiagnosticBundle() } catch (reason) { error.value = `无法生成诊断预览：${messageOf(reason)}` }
}

async function applyRecommendation(action: DiagnosticRecommendation['action']) {
  if (action === 'interfaces') view.value = 'interfaces'
  if (action === 'probe') document.querySelector<HTMLInputElement>('.probe-target input')?.focus()
  if (action === 'export') await previewDiagnostics()
}

function recommendationActionText(action: DiagnosticRecommendation['action']) {
  return { interfaces: '检查网卡', probe: '重新探测', export: '准备诊断包', none: '' }[action]
}

function saveRuleSettings(): Promise<void> {
  const snapshot = {
    schema_version: 2,
    initialized: true,
    default_outbound: defaultOutbound.value,
    rules: customRules.value.map(rule => ({ id: rule.id, name: rule.name, type: rule.type, values: rule.values, action: rule.action, enabled: rule.enabled })),
    rule_order: [...ruleOrder.value],
  } as rulesettings.Settings
  ruleSettingsSave = ruleSettingsSave.then(async () => {
    await SetRuleSettings(snapshot)
    ruleSettingsSaveError = undefined
    localStorage.removeItem('winrouter.customRules.v1')
    localStorage.removeItem('winrouter.ruleOrder.v1')
    localStorage.removeItem('winrouter.default-outbound.v1')
  }).catch(reason => { ruleSettingsSaveError = reason; error.value = `无法保存规则设置：${messageOf(reason)}` })
  return ruleSettingsSave
}

async function flushRuleSettings(): Promise<void> {
  await ruleSettingsSave
  if (ruleSettingsSaveError) throw ruleSettingsSaveError
}

function resetRuleForm() {
  ruleForm.value = { id: '', name: '', type: 'domain-suffix', valuesText: '', action: 'a', enabled: true }
  resetSRSForm()
  ruleFormOpen.value = false
}

function editCustomRule(rule: CustomRule) {
  ruleForm.value = { ...rule, valuesText: rule.values.join('\n') }
  ruleFormOpen.value = true
}

function moveMasterRule(key: string, direction: -1 | 1) {
  const index = ruleOrder.value.indexOf(key)
  const target = index + direction
  if (index < 0 || target < 0 || target >= ruleOrder.value.length) return
  const next = [...ruleOrder.value]
  const [item] = next.splice(index, 1)
  next.splice(target, 0, item)
  ruleOrder.value = next
  void saveRuleSettings().then(refreshRulePreview)
}

function toggleCustomRule(rule: CustomRule) {
  rule.enabled = !rule.enabled
  void saveRuleSettings().then(refreshRulePreview)
}

async function submitRule() {
  if (ruleForm.value.type === 'rule-set') {
    await submitSRSSource()
    return
  }
  const type = ruleForm.value.type as InlineRuleType
  const values = ruleForm.value.valuesText.split(/\r?\n/).map(value => normalizeRuleValue(type, value)).filter(Boolean)
  if (!values.length) throw new Error('至少填写一个匹配值')
  const next: CustomRule = { id: ruleForm.value.id || crypto.randomUUID(), name: ruleForm.value.name, type, values: [...new Set(values)], action: ruleForm.value.action, enabled: ruleForm.value.enabled }
  const index = customRules.value.findIndex(rule => rule.id === next.id)
  next.enabled = next.enabled !== false
  if (index >= 0) customRules.value[index] = next
  else customRules.value.push(next)
  if (!ruleOrder.value.includes(next.id)) ruleOrder.value.push(next.id)
  await saveRuleSettings()
  resetRuleForm()
  await refreshRulePreview()
}

async function deleteCustomRule(id: string) {
  customRules.value = customRules.value.filter(rule => rule.id !== id)
  ruleOrder.value = ruleOrder.value.filter(key => key !== id)
  await saveRuleSettings()
  await refreshRulePreview()
}

function ruleTypeText(type: InlineRuleType) {
  return ({ 'domain-suffix': '域名后缀', domain: '精确域名', ip: 'IP/CIDR', 'process-name': '进程', 'process-path': '进程路径' } as Record<InlineRuleType, string>)[type]
}

function normalizeRuleValue(type: InlineRuleType, raw: string) {
  const value = raw.trim()
  if (type === 'domain' || type === 'domain-suffix') return value.toLowerCase().replace(/^\.+|\.+$/g, '')
  if (type === 'process-name' || type === 'process-path') return value.toLowerCase()
  return value
}

async function refreshRulePreview() {
  rulePreviewError.value = ''
  try { await flushRuleSettings(); const input = buildConfig(); rulePreview.value = await PreviewCoreRules(input); processRuleStatuses.value = await InspectProcessRules(input) }
  catch (reason) { rulePreview.value = []; processRuleStatuses.value = []; rulePreviewError.value = messageOf(reason) }
}

async function exportDiagnostics() {
  busy.value = true
  try { const path = await ExportDiagnosticBundle(); if (path) { notice.value = `诊断包已导出：${path}`; addLog('info', '诊断包已完成脱敏并导出。') } } catch (reason) { error.value = `导出失败：${messageOf(reason)}` } finally { busy.value = false }
}

async function updateAutostart() {
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
	} finally { autostartBusy.value = false }
}

function openNewProxyForm() {
  resetProxyForm()
  proxyFormOpen.value = true
}

function selectProtocol(type: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan') {
  if (proxyForm.value.type === type) return
  proxyForm.value.type = type
  if (type === 'trojan') {
    proxyForm.value.tls_enabled = true
  }

  if (proxyForm.value.id) {
    if (type === proxyForm.value.original_type) {
      proxyForm.value.has_saved_secret = proxyForm.value.original_has_secret
      proxyForm.value.replace_secret = !proxyForm.value.original_has_secret
      proxyForm.value.clear_secret = false
    } else {
      // Switched away from original protocol: old DPAPI secret cannot be reused across different protocols
      proxyForm.value.has_saved_secret = false
      proxyForm.value.replace_secret = true
      proxyForm.value.clear_secret = false
      // Clear protocol-specific fields of previous protocol
      if (type === 'vmess' || type === 'vless') {
        proxyForm.value.password = ''
      } else if (type === 'shadowsocks' || type === 'trojan') {
        proxyForm.value.uuid = ''
        proxyForm.value.flow = ''
      } else if (type === 'http') {
        proxyForm.value.uuid = ''
        proxyForm.value.flow = ''
      }
    }
  }
  proxyFieldErrors.value = {}
}

function resetProxyForm() {
  proxyForm.value = {
    id: '',
    original_type: 'http',
    original_has_secret: false,
    name: '',
    type: 'http',
    server: '',
    port: 8080,
    egress: 'b',
    username: '',
    password: '',
    method: 'aes-256-gcm',
    uuid: '',
    flow: '',
    tls_enabled: false,
    tls_server_name: '',
    tls_insecure: false,
    tls_alpn: '',
    transport_type: 'tcp',
    transport_path: '',
    transport_host: '',
    has_saved_secret: false,
    replace_secret: false,
    clear_secret: false,
  }
  proxyFieldErrors.value = {}
  proxyFormOpen.value = false
}

function editProxyNode(node: nodes.Node) {
  const hasSecret = Boolean(node.has_secret || node.has_password)
  const nodeType = ((node.type as any) || 'http') as 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  proxyForm.value = {
    id: node.id,
    original_type: nodeType,
    original_has_secret: hasSecret,
    name: node.name,
    type: nodeType,
    server: node.server,
    port: node.port,
    egress: (node.egress as any) || 'b',
    username: node.authentication?.username || node.username || '',
    password: '',
    method: node.authentication?.method || (node.type === 'shadowsocks' ? node.username : 'aes-256-gcm') || 'aes-256-gcm',
    uuid: '',
    flow: node.authentication?.flow || '',
    tls_enabled: node.tls?.enabled || (node.type === 'trojan'),
    tls_server_name: node.tls?.server_name || '',
    tls_insecure: node.tls?.insecure || false,
    tls_alpn: node.tls?.alpn ? node.tls.alpn.join(', ') : '',
    transport_type: (node.transport?.type === 'ws' ? 'ws' : 'tcp'),
    transport_path: node.transport?.path || '',
    transport_host: node.transport?.host || '',
    has_saved_secret: hasSecret,
    replace_secret: !hasSecret,
    clear_secret: false,
  }
  proxyFieldErrors.value = {}
  proxyFormOpen.value = true
}

function validateProxyForm(): boolean {
  proxyFieldErrors.value = {}
  const f = proxyForm.value
  const protoMeta = proxyProtocols.find(p => p.value === f.type)
  let isValid = true

  if (!f.name || !f.name.trim()) {
    proxyFieldErrors.value.name = '请输入节点名称'
    isValid = false
  } else if (f.name.length > 80) {
    proxyFieldErrors.value.name = '节点名称最多 80 个字符'
    isValid = false
  }

  if (!f.server || !f.server.trim()) {
    proxyFieldErrors.value.server = '请输入服务器地址（IPv4 或域名）'
    isValid = false
  } else if (f.server.includes(' ')) {
    proxyFieldErrors.value.server = '服务器地址不能包含空格'
    isValid = false
  }

  if (!f.port || f.port < 1 || f.port > 65535) {
    proxyFieldErrors.value.port = '端口必须在 1-65535 之间'
    isValid = false
  }

  const isProtocolChanged = Boolean(f.id && f.type !== f.original_type)
  const hasSavedSecretForCurrentProtocol = f.has_saved_secret && !isProtocolChanged
  const needsSecret = !f.id || f.replace_secret || !hasSavedSecretForCurrentProtocol
  if (needsSecret && !f.clear_secret) {
    if (f.type === 'shadowsocks' && !f.password) {
      proxyFieldErrors.value.password = 'Shadowsocks 密码为必填项'
      isValid = false
    } else if (f.type === 'trojan' && !f.password) {
      proxyFieldErrors.value.password = 'Trojan 密码为必填项'
      isValid = false
    } else if (f.type === 'vmess' || f.type === 'vless') {
      if (!f.uuid || !f.uuid.trim()) {
        proxyFieldErrors.value.uuid = `${f.type.toUpperCase()} UUID 为必填项`
        isValid = false
      } else {
        const uuidRegex = /^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/
        if (!uuidRegex.test(f.uuid.trim())) {
          proxyFieldErrors.value.uuid = 'UUID 格式无效，必须为 36 位标准 UUID（如 8-4-4-4-12）'
          isValid = false
        }
      }
    }
  }

  const isTLSEnabled = Boolean(protoMeta?.supportsTLS && (protoMeta.forceTLS || f.tls_enabled))
  if (isTLSEnabled) {
    if (f.tls_server_name && (f.tls_server_name.includes(' ') || f.tls_server_name.length > 253)) {
      proxyFieldErrors.value.tls_server_name = 'TLS Server Name 不能包含空格且长度不超过 253'
      isValid = false
    }
    if (f.tls_alpn) {
      const tokens = f.tls_alpn.split(/[,;\s]+/).map(s => s.trim()).filter(Boolean)
      for (const tok of tokens) {
        if (tok.length > 32) {
          proxyFieldErrors.value.tls_alpn = `ALPN 单项 "${tok}" 长度超过 32 字符`
          isValid = false
          break
        }
      }
    }
  }

  const isWSEnabled = Boolean(protoMeta?.supportsTransport && f.transport_type === 'ws')
  if (isWSEnabled) {
    if (f.transport_path) {
      if (!f.transport_path.startsWith('/')) {
        proxyFieldErrors.value.transport_path = 'WebSocket Path 必须以 "/" 开头（例如 /ws）'
        isValid = false
      } else if (f.transport_path.length > 2048) {
        proxyFieldErrors.value.transport_path = 'WebSocket Path 长度不能超过 2048 字符'
        isValid = false
      }
    }
    if (f.transport_host && (f.transport_host.includes(' ') || f.transport_host.length > 253)) {
      proxyFieldErrors.value.transport_host = 'WebSocket Host 不能包含空格且长度不超过 253'
      isValid = false
    }
  }

  return isValid
}

async function saveProxyNode() {
  error.value = ''
  notice.value = ''
  if (!validateProxyForm()) {
    error.value = '请更正表单中的输入错误。'
    return
  }
  if (isRunning.value) {
    error.value = '核心运行中，禁止修改或添加节点；请先停止核心。'
    return
  }
  busy.value = true
  try {
    const f = proxyForm.value
    const protoMeta = proxyProtocols.find(p => p.value === f.type)
    const alpnTokens = f.tls_alpn ? f.tls_alpn.split(/[,;\s]+/).map(s => s.trim()).filter(Boolean) : undefined
    const isTLSEnabled = Boolean(protoMeta?.supportsTLS && (protoMeta.forceTLS || f.tls_enabled))
    const isWSEnabled = Boolean(protoMeta?.supportsTransport && f.transport_type === 'ws')

    const authInput: nodes.AuthenticationInput = {
      username: f.type === 'http' ? (f.username?.trim() || undefined) : undefined,
      password: (f.type === 'http' || f.type === 'shadowsocks' || f.type === 'trojan') ? (f.password || undefined) : undefined,
      method: f.type === 'shadowsocks' ? (f.method || undefined) : undefined,
      uuid: (f.type === 'vmess' || f.type === 'vless') ? (f.uuid?.trim() || undefined) : undefined,
      flow: (protoMeta?.supportsFlow && f.type === 'vless') ? (f.flow || undefined) : undefined,
    }

    const tlsInput: nodes.TLSInput | undefined = isTLSEnabled ? {
      enabled: true,
      server_name: f.tls_server_name?.trim() || undefined,
      insecure: f.tls_insecure || false,
      alpn: alpnTokens && alpnTokens.length ? alpnTokens : undefined,
    } : undefined

    const transportInput: nodes.TransportInput | undefined = isWSEnabled ? {
      type: 'ws',
      path: f.transport_path?.trim() || undefined,
      host: f.transport_host?.trim() || undefined,
    } : undefined

    const hasNewSecret = Boolean(authInput.password || authInput.uuid)
    const isProtocolChanged = Boolean(f.id && f.type !== f.original_type)
    const shouldClearSecret = f.clear_secret || (isProtocolChanged && !hasNewSecret)

    const input = {
      id: f.id || undefined,
      name: f.name.trim(),
      type: f.type,
      server: f.server.trim(),
      port: f.port,
      egress: f.egress,
      clear_secret: shouldClearSecret,
      authentication: authInput,
      tls: tlsInput,
      transport: transportInput,
    } as unknown as nodes.Input

    if (input.id) await UpdateProxyNode(input)
    else await AddProxyNode(input)
    proxyNodes.value = await ListProxyNodes()
    if (input.id) {
      if (shouldClearSecret) notice.value = '代理节点已更新，已清除旧凭据。'
      else if (f.replace_secret || hasNewSecret) notice.value = '代理节点已更新，新凭据已安全保存。'
      else notice.value = '代理节点已更新，已保留原有加密凭据。'
    } else {
      notice.value = '代理节点已添加并安全保存。'
    }
    resetProxyForm()
  } catch (reason) {
    error.value = `无法保存代理节点：${messageOf(reason)}`
  } finally { busy.value = false }
}

function openImportBox() {
  proxyImportURI.value = ''
  proxyImportError.value = ''
  proxyImportOpen.value = true
}

function closeImportBox() {
  proxyImportOpen.value = false
  proxyImportURI.value = ''
  proxyImportError.value = ''
}

function safeBase64Decode(raw: string): string {
  let str = raw.trim().replace(/-/g, '+').replace(/_/g, '/')
  while (str.length % 4 !== 0) {
    str += '='
  }
  const binary = atob(str)
  const bytes = new Uint8Array(binary.length)
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i)
  }
  return new TextDecoder('utf-8').decode(bytes)
}

function parseProxyURI(uri: string) {
  const trimmed = uri.trim()
  if (!trimmed) throw new Error('链接内容为空')

  if (trimmed.startsWith('ss://')) {
    let body = trimmed.slice(5)
    let tag = ''
    const hashIdx = body.indexOf('#')
    if (hashIdx !== -1) {
      tag = decodeURIComponent(body.slice(hashIdx + 1))
      body = body.slice(0, hashIdx)
    }
    let method = 'aes-256-gcm'
    let password = ''
    let server = ''
    let port = 8388

    if (body.includes('@')) {
      const atIdx = body.indexOf('@')
      const userInfoEncoded = body.slice(0, atIdx)
      const serverPort = body.slice(atIdx + 1)
      let userInfo = ''
      try {
        userInfo = safeBase64Decode(userInfoEncoded)
      } catch {
        userInfo = userInfoEncoded
      }
      const colonIdx = userInfo.indexOf(':')
      if (colonIdx !== -1) {
        method = userInfo.slice(0, colonIdx)
        password = userInfo.slice(colonIdx + 1)
      }
      const lastColon = serverPort.lastIndexOf(':')
      if (lastColon !== -1) {
        server = serverPort.slice(0, lastColon)
        port = parseInt(serverPort.slice(lastColon + 1), 10) || 8388
      } else {
        server = serverPort
      }
    } else {
      let decoded = ''
      try {
        decoded = safeBase64Decode(body)
      } catch {
        throw new Error('无效的 Shadowsocks base64 编码')
      }
      const atIdx = decoded.indexOf('@')
      if (atIdx !== -1) {
        const userInfo = decoded.slice(0, atIdx)
        const serverPort = decoded.slice(atIdx + 1)
        const colonIdx = userInfo.indexOf(':')
        if (colonIdx !== -1) {
          method = userInfo.slice(0, colonIdx)
          password = userInfo.slice(colonIdx + 1)
        }
        const lastColon = serverPort.lastIndexOf(':')
        if (lastColon !== -1) {
          server = serverPort.slice(0, lastColon)
          port = parseInt(serverPort.slice(lastColon + 1), 10) || 8388
        } else {
          server = serverPort
        }
      }
    }

    const methodLower = method.toLowerCase().trim()
    if (!ssMethods.includes(methodLower)) {
      throw new Error(`不支持的 Shadowsocks 加密方法 "${method}"，后端当前仅支持: ${ssMethods.join(', ')}`)
    }

    return {
      name: tag || `${server}:${port}`,
      type: 'shadowsocks' as const,
      server,
      port,
      method: methodLower,
      password,
    }
  }

  if (trimmed.startsWith('vmess://')) {
    const b64 = trimmed.slice(8)
    let jsonStr = ''
    try {
      jsonStr = safeBase64Decode(b64)
    } catch {
      throw new Error('无效的 VMess base64 编码')
    }
    const data = JSON.parse(jsonStr)
    const vmessAlpn = (data.alpn || '').trim()
    return {
      name: data.ps || `${data.add}:${data.port}`,
      type: 'vmess' as const,
      server: data.add || '',
      port: Number(data.port) || 443,
      uuid: data.id || '',
      tls_enabled: data.tls === 'tls' || data.tls === '1' || data.tls === 'true',
      tls_server_name: data.sni || data.host || data.add || '',
      transport_type: (data.net === 'ws' ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: data.path || '',
      transport_host: data.host || '',
      tls_alpn: vmessAlpn.toLowerCase() === 'default' ? '' : vmessAlpn,
    }
  }

  if (trimmed.startsWith('vless://')) {
    const url = new URL(trimmed)
    const uuid = url.username
    const server = url.hostname
    const port = parseInt(url.port, 10) || 443
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    const params = url.searchParams
    const security = (params.get('security') || '').toLowerCase()
    if (security === 'reality') {
      throw new Error('暂不支持 VLESS Reality 协议链接（需要服务端与客户端 Reality 公钥及 Short ID 支持）')
    }
    const isTLS = security === 'tls'
    const isWS = params.get('type') === 'ws'
    const alpn = (params.get('alpn') || '').trim()
    return {
      name: tag || `${server}:${port}`,
      type: 'vless' as const,
      server,
      port,
      uuid,
      flow: params.get('flow') || '',
      tls_enabled: isTLS,
      tls_server_name: params.get('sni') || params.get('peer') || server || '',
      tls_insecure: params.get('allowInsecure') === '1' || params.get('insecure') === '1',
      // Xray uses "default" as a sentinel meaning no explicit ALPN. It is
      // not a literal protocol name and must not be sent to sing-box.
      tls_alpn: alpn.toLowerCase() === 'default' ? '' : alpn,
      transport_type: (isWS ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: params.get('path') || '',
      transport_host: params.get('host') || '',
    }
  }

  if (trimmed.startsWith('trojan://')) {
    const url = new URL(trimmed)
    const password = url.username
    const server = url.hostname
    const port = parseInt(url.port, 10) || 443
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    const params = url.searchParams
    const isWS = params.get('type') === 'ws'
    const trojanAlpn = (params.get('alpn') || '').trim()
    return {
      name: tag || `${server}:${port}`,
      type: 'trojan' as const,
      server,
      port,
      password,
      tls_enabled: true,
      tls_server_name: params.get('sni') || params.get('peer') || server || '',
      tls_insecure: params.get('allowInsecure') === '1' || params.get('insecure') === '1',
      tls_alpn: trojanAlpn.toLowerCase() === 'default' ? '' : trojanAlpn,
      transport_type: (isWS ? 'ws' : 'tcp') as 'tcp' | 'ws',
      transport_path: params.get('path') || '',
      transport_host: params.get('host') || '',
    }
  }

  if (trimmed.startsWith('http://') || trimmed.startsWith('https://')) {
    const url = new URL(trimmed)
    const tag = decodeURIComponent(url.hash.replace(/^#/, ''))
    return {
      name: tag || `${url.hostname}:${url.port || 8080}`,
      type: 'http' as const,
      server: url.hostname,
      port: parseInt(url.port, 10) || 8080,
      username: url.username ? decodeURIComponent(url.username) : '',
      password: url.password ? decodeURIComponent(url.password) : '',
    }
  }

  throw new Error('不支持的链接协议，支持 ss://, vmess://, vless://, trojan://, http://')
}

function applyImportURI() {
  proxyImportError.value = ''
  try {
    const parsed = parseProxyURI(proxyImportURI.value)
    resetProxyForm()
    proxyForm.value = {
      id: '',
      original_type: parsed.type,
      original_has_secret: false,
      name: parsed.name || '',
      type: parsed.type,
      server: parsed.server,
      port: parsed.port,
      egress: 'b',
      username: (parsed as any).username || '',
      password: (parsed as any).password || '',
      method: (parsed as any).method || 'aes-256-gcm',
      uuid: (parsed as any).uuid || '',
      flow: (parsed as any).flow || '',
      tls_enabled: (parsed as any).tls_enabled || (parsed.type === 'trojan'),
      tls_server_name: (parsed as any).tls_server_name || '',
      tls_insecure: (parsed as any).tls_insecure || false,
      tls_alpn: (parsed as any).tls_alpn || '',
      transport_type: (parsed as any).transport_type || 'tcp',
      transport_path: (parsed as any).transport_path || '',
      transport_host: (parsed as any).transport_host || '',
      has_saved_secret: false,
      replace_secret: true,
      clear_secret: false,
    }
    proxyFormOpen.value = true
    closeImportBox()
    notice.value = `已从链接导入节点“${proxyForm.value.name}”，请核对并保存。`
  } catch (reason) {
    proxyImportError.value = `导入解析失败：${messageOf(reason)}`
  }
}

async function copyProxyNodeInfo(node: nodes.Node) {
  try {
    let text = `${node.name}\n`
    text += `类型: ${node.type === 'http' ? 'HTTP CONNECT' : node.type.toUpperCase()}\n`
    text += `服务器: ${node.server}:${node.port}\n`
    text += `出口: 接口 ${node.egress.toUpperCase()}\n`
    if (node.authentication?.username) text += `用户名: ${node.authentication.username}\n`
    if (node.authentication?.method) text += `加密方法: ${node.authentication.method}\n`
    if (node.authentication?.flow) text += `Flow: ${node.authentication.flow}\n`
    if (node.tls?.enabled || node.type === 'trojan') {
      text += `TLS: 启用`
      if (node.tls?.server_name) text += ` (SNI: ${node.tls.server_name})`
      if (node.tls?.alpn?.length) text += ` (ALPN: ${node.tls.alpn.join(', ')})`
      if (node.tls?.insecure) text += ` (Insecure)`
      text += `\n`
    }
    if (node.transport?.type === 'ws') {
      text += `传输: WebSocket`
      if (node.transport.path) text += ` (Path: ${node.transport.path})`
      if (node.transport.host) text += ` (Host: ${node.transport.host})`
      text += `\n`
    }
    await navigator.clipboard.writeText(text)
    notice.value = `已复制节点“${node.name}”配置摘要到剪贴板。`
  } catch (reason) {
    error.value = `复制失败：${messageOf(reason)}`
  }
}

function formatProxySummary(node: nodes.Node): string[] {
  const parts: string[] = []
  if (node.type === 'http') {
    parts.push('HTTP CONNECT')
    if (node.authentication?.username || node.username) {
      parts.push(`用户: ${node.authentication?.username || node.username}`)
    }
  } else if (node.type === 'shadowsocks') {
    parts.push(`SS · ${node.authentication?.method || node.username || 'aes-256-gcm'}`)
  } else if (node.type === 'vmess') {
    parts.push('VMess')
  } else if (node.type === 'vless') {
    parts.push('VLESS')
    if (node.authentication?.flow) {
      parts.push(node.authentication.flow)
    }
  } else if (node.type === 'trojan') {
    parts.push('Trojan')
  } else {
    parts.push(node.type.toUpperCase())
  }

  if (node.type === 'trojan' || node.tls?.enabled) {
    let tlsText = 'TLS'
    const sub: string[] = []
    if (node.tls?.server_name) sub.push(`SNI: ${node.tls.server_name}`)
    if (node.tls?.alpn?.length) sub.push(`ALPN: ${node.tls.alpn.join(',')}`)
    if (node.tls?.insecure) sub.push('Insecure')
    if (sub.length) tlsText += ` (${sub.join(' · ')})`
    parts.push(tlsText)
  }

  if (node.transport?.type === 'ws') {
    let wsText = 'WS'
    const wsSub: string[] = []
    if (node.transport.path) wsSub.push(node.transport.path)
    if (node.transport.host) wsSub.push(`host: ${node.transport.host}`)
    if (wsSub.length) wsText += ` (${wsSub.join(' · ')})`
    parts.push(wsText)
  } else if (node.type === 'vmess' || node.type === 'vless' || node.type === 'trojan') {
    parts.push('TCP')
  }

  return parts
}

async function selectProxyNode(id: string) {
  if (isRunning.value) {
    error.value = '核心运行中，禁止切换代理节点；请先停止核心。'
    return
  }
  try { await SelectProxyNode(id); proxyNodes.value = await ListProxyNodes(); notice.value = '默认代理节点已更新。' }
  catch (reason) { error.value = `无法选择代理节点：${messageOf(reason)}` }
}

async function deleteProxyNode(node: nodes.Node) {
  if (isRunning.value) {
    error.value = '核心运行中，禁止删除代理节点；请先停止核心。'
    return
  }
  if (!window.confirm(`删除代理节点“${node.name}”？此操作会同时删除其加密凭据。`)) return
  try { await DeleteProxyNode(node.id); proxyNodes.value = await ListProxyNodes(); notice.value = '代理节点及其加密凭据已删除。'; if (proxyForm.value.id === node.id) resetProxyForm() }
  catch (reason) { error.value = `无法删除代理节点：${messageOf(reason)}` }
}

function setMode(mode: 'direct-split' | 'proxy-split') {
  if (mode === selectedMode.value) return
  error.value = ''
  if (isRunning.value) {
    error.value = '请先停止当前核心，再切换运行模式。'
    return
  }
  if (mode === 'proxy-split') {
    if (!proxyNodes.value.some(node => node.selected)) {
      error.value = '请先添加并选择一个代理节点。'
      view.value = 'proxy'
      return
    }
    if (!window.confirm('代理模式会将非国内公网流量严格送入所选代理。代理故障时不会回退为直连，确认切换吗？')) return
  }
  selectedMode.value = mode
  notice.value = mode === 'proxy-split' ? '已切换为代理模式，启动前仍会执行完整校验。' : '已切换为双网卡直连模式。'
}

function getBootstrapDNSServer(): string {
  return dnsSettings.value.domestic?.server || '223.5.5.5'
}

async function testProxy(node: nodes.Node) {
  testingProxyID.value = node.id
  error.value = ''
  try {
    const result = await TestProxyNode(node.id, getBootstrapDNSServer())
    proxyTests.value = { ...proxyTests.value, [node.id]: result }
  } catch (reason) {
    error.value = `节点测试失败：${messageOf(reason)}`
  } finally { testingProxyID.value = '' }
}

async function speedTestAllProxies() {
  testingAllProxies.value = true; error.value = ''
  try {
    const results = await SpeedTestProxyNodes(getBootstrapDNSServer())
    const next = { ...proxyTests.value }
    for (const result of results) next[result.node_id] = result
    proxyTests.value = next
    notice.value = `测速完成：${results.filter(item => item.available).length}/${results.length} 个节点可用。`
  } catch (reason) { error.value = `批量测速失败：${messageOf(reason)}` }
  finally { testingAllProxies.value = false }
}

async function toggleProxyFavorite(node: nodes.Node) {
  try { await SetProxyNodeFavorite(node.id, !node.favorite); proxyNodes.value = await ListProxyNodes() }
  catch (reason) { error.value = `无法更新收藏：${messageOf(reason)}` }
}

function resetSubscriptionForm() { subscriptionForm.value = { id: '', name: '', url: '' }; subscriptionFormOpen.value = false }
function editSubscription(item: subscriptions.Subscription) { subscriptionForm.value = { id: item.id, name: item.name, url: '' }; subscriptionFormOpen.value = true }
async function saveSubscription() {
  busy.value = true; error.value = ''
  try {
    if (subscriptionForm.value.id) await UpdateSubscription(subscriptionForm.value)
    else await AddSubscription(subscriptionForm.value)
    subscriptionList.value = await ListSubscriptions(); resetSubscriptionForm(); notice.value = '订阅定义已安全保存。'
  } catch (reason) { error.value = `无法保存订阅：${messageOf(reason)}` } finally { busy.value = false }
}
async function refreshSubscription(item: subscriptions.Subscription) {
  refreshingSubscriptionID.value = item.id; error.value = ''
  try { await RefreshSubscription(item.id); subscriptionList.value = await ListSubscriptions(); proxyNodes.value = await ListProxyNodes(); notice.value = '订阅已校验并原子更新。' }
  catch (reason) { subscriptionList.value = await ListSubscriptions(); error.value = `订阅更新失败，已保留原节点：${messageOf(reason)}` }
  finally { refreshingSubscriptionID.value = '' }
}
async function deleteSubscription(item: subscriptions.Subscription) {
  if (!window.confirm(`删除订阅“${item.name}”及其节点？`)) return
  try { await DeleteSubscription(item.id); subscriptionList.value = await ListSubscriptions(); proxyNodes.value = await ListProxyNodes(); notice.value = '订阅及其节点已删除。' }
  catch (reason) { error.value = `无法删除订阅：${messageOf(reason)}` }
}

async function saveSelection() {
  error.value = ''
  notice.value = ''
  if (!selectionValid.value) {
    error.value = selectedA.value === selectedB.value ? '出口 A 和出口 B 不能使用同一接口。' : '请选择两块可用接口。'
    return
  }
  const changing = hasSavedSelection.value && (snapshot.value?.interface_a.match?.adapter.guid !== selectedA.value || snapshot.value?.interface_b.match?.adapter.guid !== selectedB.value)
  if (changing && !window.confirm('更改网卡会使当前策略失效。确认保存新的出口网卡吗？')) return
  busy.value = true
  try {
    const next = await SelectInterfaces(selectedA.value, selectedB.value)
    syncSelection(next)
    notice.value = '网卡选择已保存，预检信息已更新。'
    addLog('info', `已选择出口 A：${selectedAdapterA.value?.friendly_name}；出口 B：${selectedAdapterB.value?.friendly_name}`)
    view.value = 'overview'
  } catch (reason) {
    error.value = `无法保存网卡：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally { busy.value = false }
}

function buildConfig(): config.MVPConfig {
  const interfaceA = selectedAdapterA.value
  const interfaceB = selectedAdapterB.value
  if (!interfaceA || !interfaceB || !snapshot.value?.tun) throw new Error('接口或 TUN 前缀尚未就绪')
  return {
    schema_version: 1,
    mode: selectedMode.value,
    tun: { prefix: snapshot.value.tun.prefix, stack: 'system' },
    interface_a: { guid: interfaceA.guid, bind_interface: interfaceA.friendly_name },
    interface_b: { guid: interfaceB.guid, bind_interface: interfaceB.friendly_name },
    default_outbound: defaultOutbound.value,
    direct_prefixes: directPrefixes.value.map(item => ({ prefix: item.prefix, bind_interface: item.adapter_name })),
    rule_order: ruleOrder.value,
    custom_rules: customRules.value.filter(rule => rule.enabled).flatMap(rule => rule.values.map(value => ({ id: rule.id, name: rule.name, type: rule.type, value, action: rule.action }))),
    domestic: { cidrs: [], domain_suffixes: [] },
    dns: { domestic: { ...dnsSettings.value.domestic }, global: { ...dnsSettings.value.global } },
    ...(selectedMode.value === 'proxy-split' ? { proxy: { type: 'http', server: '0.0.0.0', port: 1 } } : {}),
    ipv6: ipv6Policy.value,
  } as unknown as config.MVPConfig
}

function updateDefaultOutbound(value: 'a' | 'b') {
  defaultOutbound.value = value
  void saveRuleSettings().then(refreshRulePreview)
}

async function startCore() {
  error.value = ''
  notice.value = ''
  if (geoRulesNotReady.value.length) {
    error.value = `以下规则集尚无可用数据：${geoRulesNotReady.value.map(source => source.name).join('、')}。请先完成更新。`
    view.value = 'rules'
    return
  }
  if (!hasSavedSelection.value || blockingDiagnostics.value.length) {
    error.value = '预检未通过。请先完成网卡选择并处理错误诊断。'
    view.value = 'interfaces'
    return
  }
  busy.value = true
  try {
    await flushRuleSettings()
    const input = buildConfig()
    if (selectedMode.value === 'proxy-split') await ValidateSelectedProxyConfiguration(input)
    else await ValidateCoreConfiguration(input)
    addLog('info', '配置预检通过，正在请求管理员权限。')
    coreStatus.value = selectedMode.value === 'proxy-split' ? await ApplySelectedProxyConfiguration(input) : await ApplyCoreConfiguration(input)
    notice.value = '分流核心已启动。'
    addLog('info', `核心已应用，进程 ${coreStatus.value.pid ?? '未知'}，代次 ${coreStatus.value.generation}`)
  } catch (reason) {
    error.value = `启动失败：${messageOf(reason)}`
    addLog('error', error.value, true)
  } finally { busy.value = false }
}

async function stopCore() {
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
  } finally { busy.value = false }
}

function formatList(values?: string[]) { return values?.length ? values.join(', ') : t('common.none') }
function statusText(status: string) { return status === 'up' ? t('status.up') : status === 'down' ? t('status.down') : status }
function levelText(level: LogLevel) { return t(`log.${level}`) }
function changeLocale(event: Event) { setLocale((event.target as HTMLSelectElement).value as Locale) }
function formatRate(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB/s`
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB/s`
  return `${Math.round(bytes)} B/s`
}
function formatBytes(bytes: number) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(1)} MB`
  return `${Math.round(bytes / 1024)} KB`
}
function displayedUsage(counter: observability.InterfaceCounter | undefined, direction: 'received' | 'transmitted') {
  if (!counter) return 0
  const baseline = usageBaselines.value[counter.guid.toLowerCase()]?.[direction] ?? 0
  const current = direction === 'received' ? counter.received_bytes : counter.transmitted_bytes
  return current >= baseline ? current - baseline : current
}
function resetInterfaceUsage(role: 'A' | 'B') {
  const counter = role === 'A' ? counterA.value : counterB.value
  if (!counter) return
  usageBaselines.value = { ...usageBaselines.value, [counter.guid.toLowerCase()]: { received: counter.received_bytes, transmitted: counter.transmitted_bytes, reset_at: new Date().toISOString() } }
  localStorage.setItem('winrouter.trafficBaselines.v1', JSON.stringify(usageBaselines.value))
  notice.value = `网卡 ${role} 的本地累计流量显示已清零。`
}
async function saveTrafficBudget() {
  budgetBusy.value = true; error.value = ''; notice.value = ''
  try {
    const status = await SetTrafficBudget({ ...budgetForm.value }) as TrafficBudgetStatus
    observations.value.traffic_budget = status
    notice.value = status.enabled ? '网卡 B 月度流量预算提醒已保存。' : '网卡 B 流量预算提醒已停用。'
  } catch (reason) { error.value = `无法保存流量预算：${messageOf(reason)}` } finally { budgetBusy.value = false }
}
async function resetTrafficBudget() {
  if (!window.confirm('将本月已统计的网卡 B 流量归零？此操作不会改变运营商统计。')) return
  budgetBusy.value = true
  try { observations.value.traffic_budget = await ResetTrafficBudget() as TrafficBudgetStatus; notice.value = '本月本地流量统计已归零。' }
  catch (reason) { error.value = `无法重置流量统计：${messageOf(reason)}` } finally { budgetBusy.value = false }
}
async function resetWindowsNetworkStack() {
  const warning = '此操作将停止分流核心，并重置 Windows Winsock、TCP/IP 和 DNS 缓存。可能清除静态 IP、网关、DNS、VPN 或虚拟网卡配置，执行期间网络会中断，完成后通常必须重启 Windows。是否继续？'
  if (!window.confirm(warning)) return
  if (!window.confirm('最后确认：立即执行系统级网络重置？执行后 WinRouter 不会自动重新启动分流核心。')) return
  networkResetBusy.value = true
  networkResetResult.value = undefined
  error.value = ''
  notice.value = ''
  try {
    const result = await ResetWindowsNetworkStack() as main.NetworkResetResult
    networkResetResult.value = result
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = result.success ? '网络组件重置已完成。请保存工作并重启 Windows。' : '网络组件重置未全部成功，请查看各步骤结果后重启 Windows。'
  } catch (reason) {
    error.value = `无法重置 Windows 网络组件：${messageOf(reason)}`
  } finally {
    networkResetBusy.value = false
  }
}
async function resetSavedInterfaces() {
  if (!window.confirm('清除已保存的出口 A/B 网卡选择？规则、DNS、代理节点和订阅都会保留。分流核心将停止。')) return
  applicationResetBusy.value = true; error.value = ''; notice.value = ''
  try {
    syncSelection(await ResetInterfaceSelection())
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = '网卡选择已重置，请重新选择出口 A 和 B。'
    view.value = 'interfaces'
  } catch (reason) { error.value = `无法重置网卡选择：${messageOf(reason)}` }
  finally { applicationResetBusy.value = false }
}
async function resetApplicationSettings() {
  if (!window.confirm('恢复应用默认设置？这会停止核心并重置网卡选择、规则、DNS 和 IPv6 策略。代理节点、订阅、规则集来源及流量统计会保留。')) return
  if (!window.confirm('重置前会自动备份当前配置。确认继续？')) return
  applicationResetBusy.value = true; error.value = ''; notice.value = ''
  try {
    const backup = await ResetApplicationSettings()
    syncSelection(await GetInterfaceSnapshot())
    const storedRules = await GetRuleSettings()
    customRules.value = Array.isArray(storedRules.rules) ? storedRules.rules as CustomRule[] : []
    ruleOrder.value = Array.isArray(storedRules.rule_order) ? [...storedRules.rule_order] : []
    defaultOutbound.value = storedRules.default_outbound as 'a' | 'b'
    dnsSettings.value = normalizeDNSSettings(await GetDNSSettings() as DNSSettings)
    ipv6Policy.value = await GetIPv6Policy() as 'block' | 'split'
    coreStatus.value = { ...coreStatus.value, state: 'stopped', pid: 0 }
    notice.value = `应用设置已恢复默认值。原配置备份于：${backup}`
  } catch (reason) { error.value = `无法恢复应用默认设置：${messageOf(reason)}` }
  finally { applicationResetBusy.value = false }
}
async function repairApplicationSettings() {
  if (!window.confirm('强制修复会备份并重建网卡选择、规则和 DNS 配置文件。代理节点、订阅及规则集来源会保留。修复后必须重启 WinRouter，是否继续？')) return
  applicationResetBusy.value = true; notice.value = ''
  try {
    const backup = await RepairApplicationSettings()
    initializationFailed.value = false
    error.value = ''
    notice.value = `配置文件已修复，原文件备份于：${backup}。请退出并重新启动 WinRouter。`
  } catch (reason) { error.value = `无法强制修复应用配置：${messageOf(reason)}` }
  finally { applicationResetBusy.value = false }
}
async function copyApplicationConfigDirectory() {
  if (!applicationConfigDirectory.value) return
  try {
    await navigator.clipboard.writeText(applicationConfigDirectory.value)
    notice.value = '配置目录路径已复制。'
  } catch (reason) { error.value = `无法复制配置目录路径：${messageOf(reason)}` }
}
function chooseDNSPreset(scope: 'domestic' | 'global') {
  const target = dnsSettings.value[scope]
  if (!target.preset_id) return
  const preset = dnsPresets.value.find(item => item.id === target.preset_id && item.scope === scope)
  if (preset) dnsSettings.value[scope] = { preset_id: preset.id, type: preset.type, server: preset.server, port: preset.port, server_name: preset.server_name || '' }
  dnsTests.value[scope] = 'idle'
}
function setCustomDNS(scope: 'domestic' | 'global') { dnsSettings.value[scope].preset_id = ''; dnsTests.value[scope] = 'idle' }
async function testDNSServer(scope: 'domestic' | 'global') {
  dnsTests.value[scope] = 'testing'
  try {
    const result = await TestDNSServer({ ...dnsSettings.value[scope] } as any) as { success: boolean }
    dnsTests.value[scope] = result.success ? 'success' : 'failed'
  } catch { dnsTests.value[scope] = 'failed' }
}
async function saveDNSSettings() {
  dnsBusy.value = true; error.value = ''; notice.value = ''
  try {
    dnsSettings.value = normalizeDNSSettings(await SetDNSSettings(dnsSettings.value as any) as DNSSettings)
    notice.value = 'DNS 设置已保存，将用于预览、启动和网络恢复。'
    await refreshRulePreview()
  } catch (reason) { error.value = `无法保存 DNS 设置：${messageOf(reason)}` }
  finally { dnsBusy.value = false }
}
async function saveIPv6Policy() {
  ipv6Busy.value = true; error.value = ''; notice.value = ''
  try { const next = await SetIPv6Policy(ipv6Policy.value); syncSelection(next); notice.value = ipv6Policy.value === 'split' ? (next.diagnostics.some(item => item.severity === 'error') ? 'IPv6 分流设置已保存，但当前网络未通过 IPv6 预检。' : 'IPv6 分流已启用并完成预检。') : 'IPv6 分流已关闭，IPv6 流量将被阻止。'; await refreshRulePreview() }
  catch (reason) { error.value = `无法更新 IPv6 策略：${messageOf(reason)}` }
  finally { ipv6Busy.value = false }
}
function trafficHeight(value: number, role: 'A' | 'B') {
  const maximum = role === 'A' ? maxTrafficRateA.value : maxTrafficRateB.value
  return `${Math.max(2, Math.round(value / maximum * 100))}%`
}

onMounted(async () => {
	try { applicationConfigDirectory.value = await GetApplicationConfigDirectory() } catch { applicationConfigDirectory.value = '' }
  try {
    const storedBaselines = JSON.parse(localStorage.getItem('winrouter.trafficBaselines.v1') || '{}')
    if (storedBaselines && typeof storedBaselines === 'object' && !Array.isArray(storedBaselines)) usageBaselines.value = storedBaselines
  } catch { localStorage.removeItem('winrouter.trafficBaselines.v1') }
  try {
    const [appStatus, interfaceSnapshot, status, recovery, observed, autostart, budget, storedDNS, presets, storedIPv6, storedRules] = await Promise.all([GetStatus(), GetInterfaceSnapshot(), GetCoreStatus(), GetRecoveryStatus(), GetObservations(), GetAutostartStatus(), GetTrafficBudgetStatus(), GetDNSSettings(), GetDNSPresets(), GetIPv6Policy(), GetRuleSettings()])
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
    customRules.value = (storedRules.initialized && Array.isArray(storedRules.rules) ? storedRules.rules : []) as CustomRule[]
    ruleOrder.value = [...(storedRules.initialized && Array.isArray(storedRules.rule_order) ? storedRules.rule_order : [])]
    defaultOutbound.value = (storedRules.initialized ? storedRules.default_outbound : 'b') as 'a' | 'b'
    proxyNodes.value = await ListProxyNodes()
    subscriptionList.value = await ListSubscriptions()
    srsPresets.value = await GetSRSPresets()
    srsSources.value = await ListSRSSources()
    for (const source of srsSources.value) if (!ruleOrder.value.includes(`srs:${source.id}`)) ruleOrder.value.push(`srs:${source.id}`)
    const validRuleKeys = new Set([...customRules.value.map(rule => rule.id), ...srsSources.value.map(source => `srs:${source.id}`)])
    ruleOrder.value = ruleOrder.value.filter(key => validRuleKeys.has(key))
    await saveRuleSettings()
    addLog('info', `应用已就绪，发现 ${interfaceSnapshot.candidates.length} 块候选网卡。`)
  } catch (reason) {
    initializationFailed.value = true
    error.value = `初始化失败：${messageOf(reason)}`
    addLog('error', error.value, true)
  }
  stopEvents = EventsOn('interfaces:changed', (event: { reason: string; snapshot: interfacemanager.Snapshot }) => {
    syncSelection(event.snapshot)
    addLog(event.snapshot.diagnostics.some(item => item.severity === 'error') ? 'warning' : 'info', '检测到网络接口变化，预检状态已刷新。')
  })
  stopRecoveryEvents = EventsOn('recovery:changed', (status: RecoveryStatus) => {
	const previous = recoveryStatus.value.state
    recoveryStatus.value = status
    if (status.state === 'waiting-for-network') addLog('warning', '选定网卡不可用，核心已安全停止并等待网络恢复。')
    if (status.state === 'recovering') addLog('info', `网络已稳定，正在执行第 ${status.attempts} 次恢复。`)
    if (status.state === 'monitoring' && status.desired_running && ['stopping', 'waiting-for-network', 'recovering', 'failed'].includes(previous)) { void refreshStatus(); addLog('info', '双出口已恢复，分流配置已重新应用。') }
    if (status.state === 'failed') addLog('error', `自动恢复失败：${status.last_error || '未知错误'}`, true)
  })
  stopTrayStartEvents = EventsOn('tray:start-core', () => { void startCore() })
  stopBudgetEvents = EventsOn('traffic-budget:reached', (threshold: 'warning' | 'limit', status: TrafficBudgetStatus) => {
    observations.value.traffic_budget = status
    addLog('warning', threshold === 'limit' ? '网卡 B 本月接口流量已达到预算。' : `网卡 B 本月接口流量已达到 ${status.warning_percent}% 提醒阈值。`)
  })
  statusTimer = window.setInterval(() => { void refreshStatus(); void refreshObservations() }, 5000)
})

onBeforeUnmount(() => {
  stopEvents?.()
  stopRecoveryEvents?.()
  stopBudgetEvents?.()
  stopTrayStartEvents?.()
  if (statusTimer) window.clearInterval(statusTimer)
})
</script>

<template>
  <div class="shell">
    <aside>
      <div class="brand"><span class="brand-mark" aria-hidden="true">W</span><span>WinRouter</span></div>
      <nav :aria-label="t('nav.label')">
        <button :class="{ active: view === 'overview' }" type="button" @click="view = 'overview'"><span aria-hidden="true">&#9636;</span>{{ t('nav.overview') }}</button>
        <button :class="{ active: view === 'monitoring' }" type="button" @click="view = 'monitoring'"><span aria-hidden="true">&#9635;</span>{{ t('nav.monitoring') }}</button>
        <button :class="{ active: view === 'interfaces' }" type="button" @click="view = 'interfaces'"><span aria-hidden="true">&#8644;</span>{{ t('nav.interfaces') }}</button>
        <button :class="{ active: view === 'rules' }" type="button" @click="view = 'rules'"><span aria-hidden="true">&#8803;</span>{{ t('nav.rules') }}</button>
        <button :class="{ active: view === 'proxy' }" type="button" @click="view = 'proxy'"><span aria-hidden="true">&#9673;</span>{{ t('nav.proxy') }}</button>
        <button :class="{ active: view === 'diagnostics' }" type="button" @click="view = 'diagnostics'"><span aria-hidden="true">&#8801;</span>{{ t('nav.diagnostics') }}</button>
        <button :class="{ active: view === 'settings' }" type="button" @click="view = 'settings'"><span aria-hidden="true">&#9881;</span>{{ t('nav.settings') }}</button>
      </nav>
      <div class="version">v{{ app.version }} · core {{ app.coreVersion }}</div>
    </aside>

    <main>
      <header>
        <div><p class="eyebrow">{{ t(selectedMode === 'proxy-split' ? 'mode.proxy' : 'mode.direct') }}</p><h1>{{ pageTitle }}</h1></div>
        <span class="state" :class="{ running: isRunning && !isRecovering, danger: coreStatus.abnormal || recoveryStatus.state === 'failed' }"><span aria-hidden="true">●</span>{{ stateLabel }}</span>
      </header>

      <div v-if="error" class="banner error" role="alert"><strong>{{ t('banner.failed') }}</strong><span>{{ error }}</span><button type="button" :aria-label="t('common.closeError')" @click="error = ''">×</button></div>
      <section v-if="initializationFailed" class="setup-callout" aria-label="启动恢复模式">
        <div><p class="section-kicker">恢复模式</p><h2>应用配置未能加载</h2><p>可备份并重建基础配置文件。代理节点、订阅和规则集来源不会被删除，修复后需要重启 WinRouter。</p><code v-if="applicationConfigDirectory">{{ applicationConfigDirectory }}</code></div>
        <button type="button" class="danger-button" :disabled="applicationResetBusy" @click="repairApplicationSettings">{{ applicationResetBusy ? '正在修复' : '强制修复配置' }}</button>
      </section>
      <div v-if="notice" class="banner success" role="status"><strong>{{ t('banner.success') }}</strong><span>{{ notice }}</span><button type="button" :aria-label="t('common.closeNotice')" @click="notice = ''">×</button></div>

      <template v-if="view === 'overview'">
        <section class="summary" aria-label="运行状态">
          <div><span>{{ t('overview.selection') }}</span><strong>{{ t(hasSavedSelection ? 'overview.completed' : 'overview.pending') }}</strong><small>{{ selectedAdapterA?.friendly_name ?? 'A' }} / {{ selectedAdapterB?.friendly_name ?? 'B' }}</small></div>
          <div><span>{{ t('overview.core') }}</span><strong>{{ t(isRunning ? 'overview.applied' : 'overview.notApplied') }}</strong><small>{{ isRunning ? `PID ${coreStatus.pid}` : t('overview.networkUnmanaged') }}</small></div>
          <div><span>{{ t('overview.verification') }}</span><strong>{{ t(isRunning ? 'overview.healthy' : 'overview.unverified') }}</strong><small>A {{ formatRate(latestTraffic.aDown + latestTraffic.aUp) }} · B {{ formatRate(latestTraffic.bDown + latestTraffic.bUp) }}</small></div>
          <div><span>{{ t('overview.ipv6') }}</span><strong>{{ t('overview.blocked') }}</strong><small>{{ t('overview.leakProtection') }}</small></div>
        </section>

        <section v-if="setupRequired" class="setup-callout">
          <div><p class="section-kicker">{{ t('overview.getStarted') }}</p><h2>{{ t('overview.chooseOutlets') }}</h2><p>{{ t('overview.outletDescription') }}</p></div>
          <button class="primary" type="button" @click="view = 'interfaces'">{{ t('overview.configure') }} <span aria-hidden="true">→</span></button>
        </section>
        <template v-else>
          <section class="mode-row" aria-label="运行模式">
            <div><p class="section-kicker">{{ t('overview.runMode') }}</p><strong>{{ t(selectedMode === 'proxy-split' ? 'overview.proxyRoute' : 'overview.directRoute') }}</strong></div>
            <div class="mode-switch"><button type="button" :class="{ active: selectedMode === 'direct-split' }" @click="setMode('direct-split')">{{ t('overview.direct') }}</button><button type="button" :class="{ active: selectedMode === 'proxy-split' }" @click="setMode('proxy-split')">{{ t('overview.proxy') }}</button></div>
          </section>
          <section class="control-band">
            <div><p class="section-kicker">{{ t('overview.coreControl') }}</p><h2>{{ t(isRunning ? 'overview.running' : coreStatus.abnormal ? 'overview.interrupted' : 'overview.stopped') }}</h2><p>{{ coreStatus.last_error || t(isRunning ? 'overview.runningDetail' : 'overview.stoppedDetail') }}</p></div>
            <button v-if="isRunning" class="stop" type="button" :disabled="busy" @click="stopCore"><span aria-hidden="true">■</span>{{ t(busy ? 'state.stopping' : 'overview.stop') }}</button>
            <button v-else class="primary" type="button" :disabled="busy" @click="startCore"><span aria-hidden="true">▶</span>{{ t(busy ? 'overview.starting' : coreStatus.abnormal ? 'overview.restart' : 'overview.start') }}</button>
          </section>
          <section class="route-grid" aria-label="出口详情">
            <article v-for="(adapter, role) in { A: selectedAdapterA, B: selectedAdapterB }" :key="role" class="route-row">
              <span class="route-letter">{{ role }}</span><div><h3>{{ adapter?.friendly_name }}</h3><p>{{ t(role === 'A' ? 'overview.domestic' : 'overview.public') }}</p></div>
              <dl><div><dt>{{ t('overview.status') }}</dt><dd>{{ statusText(adapter?.status ?? '') }}</dd></div><div><dt>{{ t('overview.address') }}</dt><dd>{{ adapter?.addresses?.[0]?.ip ?? t('common.none') }}</dd></div><div><dt>{{ t('overview.gateway') }}</dt><dd>{{ adapter?.gateways?.[0] ?? t('common.none') }}</dd></div></dl>
            </article>
          </section>
        </template>
      </template>

      <template v-else-if="view === 'monitoring'">
        <section class="monitor-summary" aria-label="连接与规则摘要">
          <div><span>{{ t('monitor.activeTcp') }}</span><strong>{{ observations.connections.active_tcp }}</strong><small>{{ observations.connections.established_tcp }} {{ t('monitor.established') }} · {{ observations.connections.listening_tcp }} {{ t('monitor.listening') }}</small></div>
          <div><span>{{ t('monitor.udp') }}</span><strong>{{ observations.connections.udp_endpoints }}</strong><small>{{ t('monitor.systemSnapshot') }}</small></div>
          <div><span>{{ t('monitor.hitA') }}</span><strong>{{ ruleHitA }}</strong><small>{{ t('monitor.logWindow') }}</small></div>
          <div><span>{{ t('monitor.hitB') }}</span><strong>{{ ruleHitB }}</strong><small>{{ t('monitor.logWindow') }}</small></div>
        </section>
        <p v-if="observationsError" class="observation-error">监控采样失败：{{ observationsError }}。当前显示上一次可用数据。</p>
        <section v-if="observations.traffic_budget.enabled" :class="['budget-status', { warning: observations.traffic_budget.warning_reached }]" aria-live="polite">
          <div><p class="section-kicker">{{ t('monitor.budget') }}</p><h2>{{ formatBytes(observations.traffic_budget.used_bytes) }} / {{ observations.traffic_budget.budget_gb }} GB</h2><p>{{ t('monitor.budgetScope') }}</p></div>
          <strong>{{ observations.traffic_budget.used_percent.toFixed(1) }}%</strong>
        </section>
        <section class="traffic-section">
          <div class="traffic-section-heading"><div><p class="section-kicker">{{ t('monitor.interfaceTraffic') }}</p><h2>各网卡实时流量</h2></div><strong>仅供参考</strong></div>
          <p class="traffic-disclaimer">数据来自 Windows 物理网卡计数器，包含该网卡上其他应用的流量；“累计清零”仅重设本机显示基线，不修改系统计数器。数值仅供参考，可能在系统重启、驱动重置或网卡重连后变化，不等同于 WinRouter 分流量或运营商账单。</p>
          <div class="traffic-panels">
            <article v-for="role in (['A', 'B'] as const)" :key="role" class="traffic-panel">
              <div class="traffic-heading"><div><span>网卡 {{ role }}</span><h3>{{ role === 'A' ? (selectedAdapterA?.friendly_name ?? '未选择') : (selectedAdapterB?.friendly_name ?? '未选择') }}</h3></div><div class="traffic-heading-actions"><small>{{ role === 'A' ? '国内与局域网出口' : '其他公网出口' }}</small><button type="button" class="secondary" :disabled="!(role === 'A' ? counterA : counterB)" @click="resetInterfaceUsage(role)">累计清零</button></div></div>
              <div class="traffic-totals">
                <div><span>清零后累计下载</span><strong>{{ formatBytes(displayedUsage(role === 'A' ? counterA : counterB, 'received')) }}</strong></div>
                <div><span>清零后累计上传</span><strong>{{ formatBytes(displayedUsage(role === 'A' ? counterA : counterB, 'transmitted')) }}</strong></div>
              </div>
              <div class="traffic-legend"><span :class="role === 'A' ? 'a-down' : 'b-down'">下载 {{ formatRate(role === 'A' ? latestTraffic.aDown : latestTraffic.bDown) }}</span><span :class="role === 'A' ? 'a-up' : 'b-up'">上传 {{ formatRate(role === 'A' ? latestTraffic.aUp : latestTraffic.bUp) }}</span></div>
              <div class="traffic-chart" :aria-label="`网卡 ${role} 流量速率趋势`">
                <div v-if="!traffic.length" class="chart-empty">{{ t('monitor.baseline') }}</div>
                <div v-for="point in traffic" :key="point.at" class="traffic-column">
                  <template v-if="role === 'A'"><i class="a-down" :style="{ height: trafficHeight(point.aDown, 'A') }"></i><i class="a-up" :style="{ height: trafficHeight(point.aUp, 'A') }"></i></template>
                  <template v-else><i class="b-down" :style="{ height: trafficHeight(point.bDown, 'B') }"></i><i class="b-up" :style="{ height: trafficHeight(point.bUp, 'B') }"></i></template>
                </div>
              </div>
              <div class="traffic-axis"><span>{{ t('monitor.earlier') }}</span><span>{{ t('monitor.now') }}</span></div>
            </article>
          </div>
        </section>
        <section class="traffic-section connection-observation">
          <div class="traffic-section-heading"><div><p class="section-kicker">连接观测</p><h2>域名与出口</h2></div><button type="button" class="secondary" :disabled="coreStatus.state === 'running'" @click="toggleConnectionObservation">{{ observations.connection_observation ? '关闭观测' : '开启观测' }}</button></div>
          <p class="traffic-disclaimer">仅在开启后读取本机 sing-box 连接 API；切换前需停止核心。</p>
          <div v-if="observations.connection_observation && observations.connection_events.length" class="connection-table">
            <div class="connection-row connection-head"><span>域名 / IP</span><span>类型</span><span>规则</span><span>出口</span><span>连接</span><span>流量</span></div>
            <div v-for="item in observations.connection_events" :key="`${item.domain}-${item.ip}-${item.outbound}`" class="connection-row"><span><strong>{{ item.domain || item.ip || '未知' }}</strong><small v-if="item.domain && item.ip && item.domain !== item.ip">{{ item.ip }}</small></span><span>{{ item.address_type || '-' }} · {{ item.protocol || '-' }}</span><span>{{ item.rule || 'final' }}</span><span>{{ item.outbound || '未知' }}</span><span>{{ item.established }} / {{ item.attempts }}</span><span>{{ formatBytes(item.bytes_down) }} ↓ · {{ formatBytes(item.bytes_up) }} ↑</span></div>
          </div>
          <p v-else-if="observations.connection_observation" class="traffic-disclaimer">尚无活动连接；核心启动后每 5 秒刷新。</p>
        </section>
        <section class="monitor-notes"><div><strong>{{ t('monitor.ruleScope') }}</strong><span>{{ t('monitor.ruleScopeDetail') }}</span></div><div><strong>{{ t('monitor.connectionScope') }}</strong><span>{{ t('monitor.connectionScopeDetail') }}</span></div></section>
      </template>

      <template v-else-if="view === 'interfaces'">
        <section class="intro"><p>{{ setupRequired ? '选择两块当前可用的物理网卡。系统会持久化接口 GUID，并在网络变化时重新核对。' : '查看当前出口，或选择其他可用网卡。修改已保存的出口前会要求确认。' }}</p></section>
        <section class="picker-grid">
          <div class="picker"><label for="interface-a"><span class="route-letter">A</span><span><strong>出口 A</strong><small>国内与局域网</small></span></label><select id="interface-a" v-model="selectedA"><option value="" disabled>选择网卡</option><option v-for="item in candidates" :key="item.adapter.guid" :value="item.adapter.guid" :disabled="!item.eligible || item.adapter.guid === selectedB">{{ item.adapter.friendly_name }} · {{ statusText(item.adapter.status) }}</option></select></div>
          <div class="picker"><label for="interface-b"><span class="route-letter alternate">B</span><span><strong>出口 B</strong><small>其他公网</small></span></label><select id="interface-b" v-model="selectedB"><option value="" disabled>选择网卡</option><option v-for="item in candidates" :key="item.adapter.guid" :value="item.adapter.guid" :disabled="!item.eligible || item.adapter.guid === selectedA">{{ item.adapter.friendly_name }} · {{ statusText(item.adapter.status) }}</option></select></div>
        </section>
        <p v-if="selectedA && selectedA === selectedB" class="field-error" role="alert">出口 A 和出口 B 不能使用同一接口。</p>
        <section class="adapter-details" aria-label="所选网卡详情">
          <article v-for="(adapter, role) in { A: selectedAdapterA, B: selectedAdapterB }" :key="role"><h3>出口 {{ role }} · {{ adapter?.friendly_name ?? '未选择' }}</h3><dl><div><dt>连接状态</dt><dd>{{ statusText(adapter?.status ?? '未知') }}</dd></div><div><dt>地址</dt><dd>{{ adapter?.addresses?.map(a => `${a.ip}/${a.prefix_length}`).join('、') || '无' }}</dd></div><div><dt>网关</dt><dd>{{ formatList(adapter?.gateways) }}</dd></div><div><dt>DNS</dt><dd>{{ formatList(adapter?.dns_servers) }}</dd></div></dl></article>
        </section>
        <section class="policy-strip"><div><span>直连前缀</span><strong>{{ directPrefixes.map(item => item.prefix).join('、') || '选择后生成' }}</strong></div><div><span>DNS 策略</span><strong>国内经 A / 全球经 B，独立缓存</strong></div><div><span>IPv6 策略</span><strong>阻止</strong></div></section>
        <section v-if="snapshot?.diagnostics?.length" class="diagnostics" aria-label="预检诊断"><h2>预检结果</h2><div v-for="item in snapshot.diagnostics" :key="item.code" :class="['diagnostic', item.severity]"><strong>{{ item.severity === 'error' ? '错误' : '提示' }} · {{ item.code }}</strong><span>{{ item.message }}</span><small>{{ item.severity === 'error' ? '请恢复网卡连接或重新选择出口后重试。' : '保存后将按当前拓扑生成配置。' }}</small></div></section>
        <div class="actions"><button type="button" class="secondary" @click="view = 'overview'">取消</button><button type="button" class="primary" :disabled="busy || !selectionValid" @click="saveSelection">{{ busy ? '正在保存' : '保存并继续' }}</button></div>
      </template>

      <template v-else-if="view === 'rules'">
        <section class="rule-outlet-map" aria-label="规则出口映射">
          <article><span>网卡 A</span><strong>{{ selectedAdapterA?.friendly_name ?? '尚未选择' }}</strong><small>{{ selectedAdapterA?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterA?.gateways?.[0] ?? '无网关' }}</small></article>
          <article><span>网卡 B</span><strong>{{ selectedAdapterB?.friendly_name ?? '尚未选择' }}</strong><small>{{ selectedAdapterB?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterB?.gateways?.[0] ?? '无网关' }}</small></article>
        </section>
        <section class="policy-strip" aria-label="当前 DNS 出口">
          <div><span>{{ selectedAdapterA?.friendly_name ?? '网卡 A' }} DNS</span><strong>{{ dnsSettings.domestic.type.toUpperCase() }} · {{ dnsSettings.domestic.server }}:{{ dnsSettings.domestic.port }}</strong></div>
          <div><span>{{ selectedAdapterB?.friendly_name ?? '网卡 B' }} DNS</span><strong>{{ dnsSettings.global.type.toUpperCase() }} · {{ dnsSettings.global.server }}:{{ dnsSettings.global.port }}</strong></div>
          <div><span>缓存策略</span><strong>按上游独立缓存</strong></div>
        </section>
        <section class="fallback-outbound" aria-label="未匹配流量兜底出口">
          <div><strong>未匹配流量兜底出口</strong><small>未命中自定义规则的流量将从此网卡发出；指定网卡 B 的规则仍优先使用 B。</small></div>
          <select :value="defaultOutbound" :disabled="selectedMode === 'proxy-split' || isRunning" @change="updateDefaultOutbound(($event.target as HTMLSelectElement).value as 'a' | 'b')">
            <option value="a">网卡 A · {{ selectedAdapterA?.friendly_name ?? '未选择' }}</option>
            <option value="b">网卡 B · {{ selectedAdapterB?.friendly_name ?? '未选择' }}</option>
          </select>
        </section>
        <section class="rules-heading">
          <div><p class="section-kicker">按实际出口配置</p><h2>用户覆盖规则</h2><p>为 IP、域名或进程指定实际网卡；系统防环路和本地网络规则始终优先。</p></div>
          <button class="primary" type="button" @click="ruleFormOpen ? resetRuleForm() : ruleFormOpen = true">{{ ruleFormOpen ? '取消' : '添加规则' }}</button>
        </section>
        <form v-if="ruleFormOpen" class="rule-form" @submit.prevent="submitRule">
          <div class="rule-form-primary">
          <label>名称<input v-model.trim="ruleForm.name" required maxlength="80" placeholder="例如：阻止广告域名"></label>
          <label>匹配类型<select v-model="ruleForm.type" :disabled="Boolean(ruleForm.id)"><option value="domain-suffix">域名后缀</option><option value="domain">精确域名</option><option value="ip">IPv4 CIDR</option><option value="process-name">进程名称</option><option value="process-path">进程完整路径</option><option value="rule-set">SRS 规则集</option></select></label>
          <label v-if="ruleForm.type === 'rule-set'">来源预设<select v-model="srsForm.preset_id" @change="chooseSRSPreset"><option value="">自定义 HTTPS 地址</option><option v-for="preset in srsPresets" :key="preset.id" :value="preset.id">{{ preset.name }}</option></select></label>
          <label>目标<select v-model="ruleForm.action"><option value="a">{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}</option><option value="b">{{ selectedAdapterB?.friendly_name ?? '网卡 B' }}</option><option value="reject">拒绝</option></select></label>
          <label class="toggle-label"><input v-model="ruleForm.enabled" type="checkbox"><span>启用规则</span></label>
          </div>
          <label v-if="ruleForm.type !== 'rule-set'" class="rule-form-values">匹配值（每行一个）<textarea v-model="ruleForm.valuesText" required rows="6" placeholder="每行填写一个域名、CIDR 或进程匹配值"></textarea></label>
          <template v-if="ruleForm.type === 'rule-set'">
            <label>数据类型<select v-model="srsForm.kind" :disabled="Boolean(srsForm.preset_id)"><option value="domain">域名集合 / geosite</option><option value="ip">IP 集合 / geoip</option></select></label>
            <label class="remote-url">固定 HTTPS 地址<input v-model.trim="srsForm.url" required type="url" :readonly="Boolean(srsForm.preset_id)" placeholder="https://example.com/rules.srs"></label>
            <label class="remote-hash">固定 SHA-256（可选） <small>{{ srsForm.preset_id ? '内置滚动源由应用记录实际 hash' : '留空则允许滚动更新；填写后锁定指定版本' }}</small><input v-model.trim="srsForm.expected_sha256" minlength="64" maxlength="64" placeholder="可留空"></label>
          </template>
          <div class="rule-form-actions"><button class="secondary" type="button" @click="resetRuleForm">取消</button><button class="primary" type="submit" :disabled="Boolean(srsBusyID)">{{ ruleForm.id ? '保存修改' : '添加规则' }}</button></div>
        </form>
        <section class="rule-master-list" aria-label="规则总表">
          <div class="rules-heading"><div><p class="section-kicker">唯一编辑入口</p><h2>分流规则总表</h2><p>规则按从上到下的顺序匹配，第一条命中后停止；网卡 A/B 区域仅展示生成结果。</p></div></div>
          <div v-if="!masterRows.length" class="outlet-rule-empty">尚未添加规则</div>
          <article v-for="(row, index) in masterRows" :key="row.key" :class="['master-rule-item', { disabled: row.kind === 'custom' ? !row.rule.enabled : !row.source.enabled }]">
            <strong class="master-rule-order">{{ index + 1 }}</strong>
            <template v-if="row.kind === 'custom'">
              <label class="master-rule-enabled"><input type="checkbox" :checked="row.rule.enabled" @change="toggleCustomRule(row.rule)"><span>{{ row.rule.enabled ? '启用' : '停用' }}</span></label>
              <div><strong>{{ row.rule.name }}</strong><small>{{ ruleTypeText(row.rule.type) }} · {{ row.rule.values.length }} 个值 · {{ row.rule.values.slice(0, 3).join('、') }}{{ row.rule.values.length > 3 ? '…' : '' }}</small></div>
              <span>{{ row.rule.action === 'a' ? `网卡 A · ${selectedAdapterA?.friendly_name ?? ''}` : row.rule.action === 'b' ? `网卡 B · ${selectedAdapterB?.friendly_name ?? ''}` : row.rule.action === 'reject' ? '拒绝' : '兜底出口' }}</span>
              <div class="proxy-actions"><button class="secondary" type="button" @click="moveMasterRule(row.key, -1)" :disabled="index === 0">上移</button><button class="secondary" type="button" @click="moveMasterRule(row.key, 1)" :disabled="index === masterRows.length - 1">下移</button><button class="secondary" type="button" @click="editCustomRule(row.rule)">编辑</button><button class="delete-button" type="button" @click="deleteCustomRule(row.rule.id)">删除</button></div>
            </template>
            <template v-else>
              <label class="master-rule-enabled"><input type="checkbox" :checked="row.source.enabled" :disabled="srsBusyID === row.source.id" @change="toggleSRSSource(row.source)"><span>{{ row.source.enabled ? '启用' : '停用' }}</span></label>
              <div><strong>{{ row.source.name }}</strong><small>{{ row.source.kind === 'domain' ? 'geosite / 域名集合' : 'geoip / IP 集合' }} · {{ row.source.applied_sha256 ? '数据集已验证' : row.source.last_error ? '下载失败，暂无缓存' : '待下载' }}</small><small>{{ formatSRSUpdatedAt(row.source) }}<template v-if="row.source.last_error && row.source.applied_sha256"> · 上次更新失败，继续使用有效缓存</template></small></div>
              <span>{{ row.source.action === 'a' ? `网卡 A · ${selectedAdapterA?.friendly_name ?? ''}` : row.source.action === 'b' ? `网卡 B · ${selectedAdapterB?.friendly_name ?? ''}` : row.source.action === 'reject' ? '拒绝' : '兜底出口' }}</span>
              <div class="proxy-actions"><button class="secondary" type="button" @click="moveMasterRule(row.key, -1)" :disabled="index === 0">上移</button><button class="secondary" type="button" @click="moveMasterRule(row.key, 1)" :disabled="index === masterRows.length - 1">下移</button><button class="secondary" type="button" @click="editSRSSource(row.source)">编辑</button><button class="secondary" type="button" :disabled="srsBusyID === row.source.id" @click="refreshSRSSource(row.source)">{{ srsBusyID === row.source.id ? '校验中' : '更新' }}</button><button v-if="!row.source.preset_id" class="delete-button" type="button" @click="deleteSRSSource(row.source)">删除</button></div>
            </template>
          </article>
        </section>
        <section class="outlet-rule-columns" aria-label="出口 DNS 设置">
          <article class="outlet-rule-group">
            <header><span>出口 A DNS</span><h3>{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}</h3><small>管理该出口使用的域名解析服务</small></header>
            <details class="outlet-dns-settings">
              <summary class="outlet-dns-heading"><strong>出口 A DNS</strong><small>{{ dnsSettings.domestic.type.toUpperCase() }} · {{ dnsSettings.domestic.server }}:{{ dnsSettings.domestic.port }}</small></summary>
              <div class="dns-row">
                <label>DNS 预设<select v-model="dnsSettings.domestic.preset_id" :disabled="dnsBusy" @change="dnsSettings.domestic.preset_id ? chooseDNSPreset('domestic') : setCustomDNS('domestic')"><option value="">自定义</option><option v-for="preset in dnsPresets.filter(item => item.scope === 'domestic')" :key="preset.id" :value="preset.id">{{ preset.name }}</option></select></label>
                <label>协议<select v-model="dnsSettings.domestic.type" :disabled="dnsBusy || Boolean(dnsSettings.domestic.preset_id)" @change="setCustomDNS('domestic')"><option value="udp">UDP</option><option value="tls">DoT</option><option value="https">DoH</option></select></label>
                <label>服务器 IP<input v-model.trim="dnsSettings.domestic.server" required :readonly="Boolean(dnsSettings.domestic.preset_id)" placeholder="1.1.1.1" @input="setCustomDNS('domestic')"></label>
                <label>端口<input v-model.number="dnsSettings.domestic.port" type="number" min="1" max="65535" required :readonly="Boolean(dnsSettings.domestic.preset_id)" @input="setCustomDNS('domestic')"></label>
                <label :class="{ 'dns-field-placeholder': dnsSettings.domestic.type === 'udp' }">TLS 域名<input v-model.trim="dnsSettings.domestic.server_name" :required="dnsSettings.domestic.type !== 'udp'" :disabled="dnsSettings.domestic.type === 'udp'" :readonly="Boolean(dnsSettings.domestic.preset_id)" :placeholder="dnsSettings.domestic.type === 'udp' ? 'UDP 不需要' : 'dns.example.com'" @input="setCustomDNS('domestic')"></label>
                <button type="button" class="dns-test" :class="dnsTests.domestic" :disabled="dnsTests.domestic === 'testing' || dnsBusy || isRunning" @click="testDNSServer('domestic')">{{ dnsTests.domestic === 'testing' ? '测试中' : dnsTests.domestic === 'success' ? '成功' : dnsTests.domestic === 'failed' ? '失败' : '测试' }}</button>
              </div>
              <div class="dns-actions"><button type="button" class="primary" :disabled="dnsBusy || isRunning" @click="saveDNSSettings">{{ dnsBusy ? '正在校验' : '保存 A DNS' }}</button></div>
            </details>
          </article>
          <article class="outlet-rule-group">
            <header><span>出口 B DNS</span><h3>{{ selectedAdapterB?.friendly_name ?? '网卡 B' }}</h3><small>管理该出口使用的域名解析服务</small></header>
            <details class="outlet-dns-settings">
              <summary class="outlet-dns-heading"><strong>出口 B DNS</strong><small>{{ dnsSettings.global.type.toUpperCase() }} · {{ dnsSettings.global.server }}:{{ dnsSettings.global.port }}</small></summary>
              <div class="dns-row">
                <label>DNS 预设<select v-model="dnsSettings.global.preset_id" :disabled="dnsBusy" @change="dnsSettings.global.preset_id ? chooseDNSPreset('global') : setCustomDNS('global')"><option value="">自定义</option><option v-for="preset in dnsPresets.filter(item => item.scope === 'global')" :key="preset.id" :value="preset.id">{{ preset.name }}</option></select></label>
                <label>协议<select v-model="dnsSettings.global.type" :disabled="dnsBusy || Boolean(dnsSettings.global.preset_id)" @change="setCustomDNS('global')"><option value="udp">UDP</option><option value="tls">DoT</option><option value="https">DoH</option></select></label>
                <label>服务器 IP<input v-model.trim="dnsSettings.global.server" required :readonly="Boolean(dnsSettings.global.preset_id)" placeholder="1.1.1.1" @input="setCustomDNS('global')"></label>
                <label>端口<input v-model.number="dnsSettings.global.port" type="number" min="1" max="65535" required :readonly="Boolean(dnsSettings.global.preset_id)" @input="setCustomDNS('global')"></label>
                <label :class="{ 'dns-field-placeholder': dnsSettings.global.type === 'udp' }">TLS 域名<input v-model.trim="dnsSettings.global.server_name" :required="dnsSettings.global.type !== 'udp'" :disabled="dnsSettings.global.type === 'udp'" :readonly="Boolean(dnsSettings.global.preset_id)" :placeholder="dnsSettings.global.type === 'udp' ? 'UDP 不需要' : 'dns.example.com'" @input="setCustomDNS('global')"></label>
                <button type="button" class="dns-test" :class="dnsTests.global" :disabled="dnsTests.global === 'testing' || dnsBusy || isRunning" @click="testDNSServer('global')">{{ dnsTests.global === 'testing' ? '测试中' : dnsTests.global === 'success' ? '成功' : dnsTests.global === 'failed' ? '失败' : '测试' }}</button>
              </div>
              <div class="dns-actions"><button type="button" class="primary" :disabled="dnsBusy || isRunning" @click="saveDNSSettings">{{ dnsBusy ? '正在校验' : '保存 B DNS' }}</button></div>
            </details>
          </article>
        </section>
        <p v-if="customRules.some(rule => rule.type.startsWith('process-'))" class="process-note">进程名称可跨重启保持匹配；同名程序请使用完整路径。子进程不会隐式继承，请为实际联网的子进程单独添加规则。路径变化或无法识别时流量仍受基础域名/IP策略约束。</p>
        <section v-if="processRuleStatuses.length" class="process-statuses" aria-label="进程规则状态">
          <div v-for="status in processRuleStatuses" :key="`${status.type}-${status.value}`" :class="status.state"><strong>{{ status.value }}</strong><span>{{ status.message }}</span><small v-if="status.paths?.length">{{ status.paths.join('、') }}</small></div>
        </section>
        <section class="final-rules">
          <div class="rules-heading"><div><p class="section-kicker">实际生成结果</p><h2>最终规则顺序</h2><p>包含基础设施、用户规则、直连前缀、内置规则及最终出口。</p></div><button class="secondary" type="button" @click="refreshRulePreview">刷新预览</button></div>
          <p v-if="rulePreviewError" class="field-error">冲突诊断：{{ rulePreviewError }}</p>
          <div class="final-rule-list"><div v-for="item in rulePreview" :key="item.position"><span>{{ item.position }}</span><strong>{{ item.category }}</strong><code>{{ item.match.join('、') }}</code><small>{{ item.action }}</small></div></div>
        </section>
      </template>

      <template v-else-if="view === 'proxy'">
        <section v-if="proxyRiskMessages.length" class="proxy-risks" aria-label="代理风险">
          <div v-for="message in proxyRiskMessages" :key="message"><strong>需要处理</strong><span>{{ message }}</span></div>
          <small>代理 endpoint 同时使用出口 B 绑定和专用 /32 规则防止再次进入代理。</small>
        </section>
        <section class="proxy-heading">
          <div>
            <p class="section-kicker">HTTP CONNECT / Shadowsocks / VMess / VLESS / Trojan</p>
            <h2>代理节点管理</h2>
            <p>节点凭据通过当前 Windows 用户的 DPAPI 加密，界面和日志不会读取或显示原文。</p>
          </div>
          <div class="proxy-heading-actions">
            <label>排序<select v-model="proxySort"><option value="favorite">收藏优先</option><option value="latency">延迟最低</option><option value="name">名称</option></select></label>
            <button class="secondary" type="button" :disabled="testingAllProxies || !proxyNodes.length" @click="speedTestAllProxies">{{ testingAllProxies ? '测速中' : '全部测速' }}</button>
            <button class="secondary" type="button" @click="proxyImportOpen ? closeImportBox() : openImportBox()">{{ proxyImportOpen ? '取消导入' : '从链接导入' }}</button>
            <button class="primary" type="button" @click="proxyFormOpen ? resetProxyForm() : openNewProxyForm()">{{ proxyFormOpen ? '取消编辑' : '添加节点' }}</button>
          </div>
        </section>

        <!-- Import from link card -->
        <section v-if="proxyImportOpen" class="proxy-import-box" aria-label="从链接导入节点">
          <div><strong>从节点分享链接导入</strong><p class="proxy-form-hint">支持 ss://、vmess://、vless://、trojan://、http:// 等单节点链接，解析后将自动填充表单供您核对与保存。</p></div>
          <textarea v-model="proxyImportURI" placeholder="粘贴节点链接，例如：vless://... 或 ss://..."></textarea>
          <p v-if="proxyImportError" class="field-error-msg">{{ proxyImportError }}</p>
          <div class="proxy-import-actions">
            <button class="secondary" type="button" @click="closeImportBox">取消</button>
            <button class="primary" type="button" :disabled="!proxyImportURI.trim()" @click="applyImportURI">解析并填入表单</button>
          </div>
        </section>

        <!-- Node form -->
        <form v-if="proxyFormOpen" class="proxy-form" @submit.prevent="saveProxyNode">
          <!-- Protocol Selector Bar -->
          <div class="protocol-selector-row">
            <span class="protocol-selector-label">选择节点协议（点击直接切换对应协议的全部专属配置字段）</span>
            <div class="protocol-switch" role="tablist" aria-label="协议类型选择">
              <button
                v-for="proto in proxyProtocols"
                :key="proto.value"
                type="button"
                :class="{ active: proxyForm.type === proto.value }"
                @click="selectProtocol(proto.value)"
              >
                {{ proto.label }}
              </button>
            </div>
          </div>

          <label>
            节点名称
            <input v-model.trim="proxyForm.name" required maxlength="80" placeholder="例如：办公代理">
            <span v-if="proxyFieldErrors.name" class="field-error-msg">{{ proxyFieldErrors.name }}</span>
          </label>
          <label>
            物理出口
            <select v-model="proxyForm.egress">
              <option value="b">接口 B（默认）</option>
              <option value="a">接口 A</option>
            </select>
          </label>
          <label>
            服务器地址
            <input v-model.trim="proxyForm.server" required placeholder="203.0.113.10 或 proxy.example.com">
            <span v-if="proxyFieldErrors.server" class="field-error-msg">{{ proxyFieldErrors.server }}</span>
          </label>
          <label>
            端口
            <input v-model.number="proxyForm.port" required type="number" min="1" max="65535">
            <span v-if="proxyFieldErrors.port" class="field-error-msg">{{ proxyFieldErrors.port }}</span>
          </label>

          <!-- Credential Management for Existing Nodes -->
          <div v-if="proxyForm.id" class="credential-status-box">
            <div class="credential-status-header">
              <div>
                <strong>{{ proxyForm.has_saved_secret ? '🔒 节点凭据状态：已加密保存' : '节点凭据状态：未配置凭据' }}</strong>
                <small v-if="proxyForm.has_saved_secret">凭据通过 Windows DPAPI 本地保护，无需重复输入。</small>
              </div>
              <div class="credential-controls">
                <label v-if="proxyForm.has_saved_secret" class="toggle">
                  <input v-model="proxyForm.replace_secret" type="checkbox" :disabled="proxyForm.clear_secret">
                  <span>{{ proxyForm.replace_secret ? '替换凭据' : '保留原凭据' }}</span>
                </label>
                <label v-if="proxyForm.type === 'http' && proxyForm.has_saved_secret" class="clear-secret">
                  <input v-model="proxyForm.clear_secret" type="checkbox" @change="proxyForm.clear_secret && (proxyForm.replace_secret = false)">
                  <span>清除已保存凭据</span>
                </label>
              </div>
            </div>
            <p v-if="proxyForm.id" class="proxy-operation-indicator">
              {{
                proxyForm.type !== proxyForm.original_type
                  ? (proxyForm.password || proxyForm.uuid
                      ? '操作预期：协议已切换，保存后将使用新凭据覆盖原有凭据。'
                      : '操作预期：协议已切换，未提供新密码/凭据，保存后将清除原协议的加密凭据。')
                  : (proxyForm.clear_secret
                      ? '操作预期：保存后将清除已保存的密码与认证凭据。'
                      : (proxyForm.replace_secret
                          ? '操作预期：保存后将使用下方新输入的凭据覆盖原有凭据。'
                          : '操作预期：保存后将保留原有加密凭据不变。'))
              }}
            </p>
          </div>

          <!-- HTTP Fields -->
          <template v-if="proxyForm.type === 'http'">
            <label>
              用户名
              <input v-model.trim="proxyForm.username" autocomplete="off" placeholder="可选用户名">
            </label>
            <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
              密码
              <input v-model="proxyForm.password" type="password" autocomplete="new-password" :disabled="proxyForm.clear_secret" :placeholder="proxyForm.id ? '输入新密码' : '可选密码'">
              <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
            </label>
          </template>

          <!-- Shadowsocks Fields -->
          <template v-else-if="proxyForm.type === 'shadowsocks'">
            <label>
              加密方法
              <select v-model="proxyForm.method" required>
                <option v-for="method in ssMethods" :key="method" :value="method">{{ method }}</option>
              </select>
            </label>
            <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
              密码
              <input v-model="proxyForm.password" type="password" autocomplete="new-password" :required="!proxyForm.id || proxyForm.replace_secret" placeholder="必填密码">
              <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
            </label>
          </template>

          <!-- VMess Fields -->
          <template v-else-if="proxyForm.type === 'vmess'">
            <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
              UUID
              <input v-model.trim="proxyForm.uuid" autocomplete="off" :required="!proxyForm.id || proxyForm.replace_secret" placeholder="例如：a8e678c0-51c0-4212-9c17-1f4bf7ec0000">
              <span v-if="proxyFieldErrors.uuid" class="field-error-msg">{{ proxyFieldErrors.uuid }}</span>
            </label>
          </template>

          <!-- VLESS Fields -->
          <template v-else-if="proxyForm.type === 'vless'">
            <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
              UUID
              <input v-model.trim="proxyForm.uuid" autocomplete="off" :required="!proxyForm.id || proxyForm.replace_secret" placeholder="例如：a8e678c0-51c0-4212-9c17-1f4bf7ec0000">
              <span v-if="proxyFieldErrors.uuid" class="field-error-msg">{{ proxyFieldErrors.uuid }}</span>
            </label>
            <label>
              Flow
              <select v-model="proxyForm.flow">
                <option value="">无 (none)</option>
                <option value="xtls-rprx-vision">xtls-rprx-vision</option>
              </select>
            </label>
          </template>

          <!-- Trojan Fields -->
          <template v-else-if="proxyForm.type === 'trojan'">
            <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
              密码
              <input v-model="proxyForm.password" type="password" autocomplete="new-password" :required="!proxyForm.id || proxyForm.replace_secret" placeholder="必填密码">
              <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
            </label>
          </template>

          <!-- TLS settings for VMess / VLESS / Trojan -->
          <template v-if="proxyForm.type === 'vmess' || proxyForm.type === 'vless' || proxyForm.type === 'trojan'">
            <label class="clear-secret">
              <input v-if="proxyForm.type !== 'trojan'" v-model="proxyForm.tls_enabled" type="checkbox">
              <input v-else type="checkbox" checked disabled>
              <span>{{ proxyForm.type === 'trojan' ? '强制启用 TLS（Trojan 协议规范）' : '启用 TLS' }}</span>
            </label>
            <template v-if="proxyForm.type === 'trojan' || proxyForm.tls_enabled">
              <label>
                TLS Server Name (SNI)
                <input v-model.trim="proxyForm.tls_server_name" placeholder="留空默认使用服务器地址">
                <span v-if="proxyFieldErrors.tls_server_name" class="field-error-msg">{{ proxyFieldErrors.tls_server_name }}</span>
              </label>
              <label>
                TLS ALPN
                <input v-model.trim="proxyForm.tls_alpn" placeholder="例如：h2, http/1.1（逗号分隔）">
                <span v-if="proxyFieldErrors.tls_alpn" class="field-error-msg">{{ proxyFieldErrors.tls_alpn }}</span>
              </label>
              <label class="clear-secret">
                <input v-model="proxyForm.tls_insecure" type="checkbox">
                <span>允许不安全证书 (Insecure)</span>
              </label>
            </template>

            <!-- Transport Settings -->
            <label>
              传输协议
              <select v-model="proxyForm.transport_type">
                <option value="tcp">TCP</option>
                <option value="ws">WebSocket (WS)</option>
              </select>
            </label>
            <template v-if="proxyForm.transport_type === 'ws'">
              <label>
                WebSocket Path
                <input v-model.trim="proxyForm.transport_path" placeholder="必须以 / 开头，例如：/ws 或 /chat">
                <span v-if="proxyFieldErrors.transport_path" class="field-error-msg">{{ proxyFieldErrors.transport_path }}</span>
              </label>
              <label>
                WebSocket Host
                <input v-model.trim="proxyForm.transport_host" placeholder="例如：proxy.example.com">
                <span v-if="proxyFieldErrors.transport_host" class="field-error-msg">{{ proxyFieldErrors.transport_host }}</span>
              </label>
            </template>
          </template>

          <div class="proxy-form-actions">
            <button class="secondary" type="button" @click="resetProxyForm">取消</button>
            <button class="primary" type="submit" :disabled="busy">{{ busy ? '正在保存' : proxyForm.id ? '保存修改' : '添加节点' }}</button>
          </div>
        </form>

        <section class="proxy-list" aria-label="代理节点列表">
          <div v-if="!proxyNodes.length" class="proxy-empty">尚未添加代理节点。</div>
          <article v-for="node in sortedProxyNodes" :key="node.id" :class="{ selected: node.selected }">
            <div class="proxy-status">
              <button class="favorite-button" type="button" :class="{ active: node.favorite }" :aria-label="node.favorite ? '取消收藏' : '收藏节点'" :title="node.favorite ? '取消收藏' : '收藏节点'" @click="toggleProxyFavorite(node)">{{ node.favorite ? '★' : '☆' }}</button>
              <span :class="['selection-dot', { active: node.selected }]" aria-hidden="true"></span>
              <div>
                <strong>{{ node.name }}</strong>
                <small>{{ node.type === 'http' ? 'HTTP CONNECT' : node.type.toUpperCase() }} · {{ node.server }}:{{ node.port }}<template v-if="node.resolved_ip && node.resolved_ip !== node.server"> → {{ node.resolved_ip }}</template></small>
              </div>
            </div>
            <div class="proxy-meta">
              <span>{{ node.egress === 'a' ? '出口 A' : '出口 B' }}</span>
              <span v-for="tag in formatProxySummary(node)" :key="tag" class="proxy-meta-tag">{{ tag }}</span>
              <span :class="['credential-tag', { none: !(node.has_secret || node.has_password) }]">
                {{ (node.has_secret || node.has_password) ? '🔒 已保存凭据' : '未设凭据' }}
              </span>
              <span v-if="proxyTests[node.id]" :class="proxyTests[node.id].available ? 'test-ok' : 'test-failed'">
                <template v-if="proxyTests[node.id].available">
                  可用 · {{ proxyTests[node.id].total_ms || proxyTests[node.id].latency_ms }} ms
                  <small v-if="proxyTests[node.id].tcp_ms"> (TCP {{ proxyTests[node.id].tcp_ms }}ms / 协议 {{ proxyTests[node.id].protocol_ms }}ms)</small>
                </template>
                <template v-else>
                  不可用 · {{ formatErrorCategory(proxyTests[node.id].error_category) }}
                </template>
              </span>
              <span v-else>尚未测试</span>
            </div>
            <div class="proxy-actions">
              <button class="secondary" type="button" :disabled="testingProxyID === node.id" @click="testProxy(node)">{{ testingProxyID === node.id ? '测试中' : '测试' }}</button>
              <button v-if="!node.selected" class="secondary" type="button" :disabled="isRunning" :title="isRunning ? '核心运行中，禁止切换节点' : '选择此节点'" @click="selectProxyNode(node.id)">选择</button>
              <span v-else class="selected-label">当前节点</span>
              <button class="secondary" type="button" title="复制节点配置摘要" @click="copyProxyNodeInfo(node)">复制</button>
              <template v-if="!node.subscription_id">
                <button class="secondary" type="button" :disabled="isRunning" :title="isRunning ? '核心运行中，禁止修改节点' : '编辑节点'" @click="editProxyNode(node)">编辑</button>
                <button class="delete-button" type="button" :disabled="isRunning" :title="isRunning ? '核心运行中，禁止删除节点' : '删除节点'" @click="deleteProxyNode(node)">删除</button>
              </template>
              <span v-else class="subscription-badge">订阅节点</span>
            </div>
          </article>
        </section>
        <section class="subscription-heading"><div><p class="section-kicker">受限 JSON</p><h2>节点订阅</h2><p>公网地址强制 HTTPS；私网或回环 IP 可使用 HTTP。不执行脚本或命令，失败时保留上一批节点。</p></div><button class="secondary" type="button" @click="subscriptionFormOpen ? resetSubscriptionForm() : subscriptionFormOpen = true">{{ subscriptionFormOpen ? '取消' : '添加订阅' }}</button></section>
        <form v-if="subscriptionFormOpen" class="subscription-form" @submit.prevent="saveSubscription">
          <label>订阅名称<input v-model.trim="subscriptionForm.name" required maxlength="80"></label>
          <label>HTTPS / 局域网 HTTP 地址<input v-model.trim="subscriptionForm.url" :required="!subscriptionForm.id" type="url" :placeholder="subscriptionForm.id ? '留空则保留加密地址' : 'https://example.com/nodes.json 或 http://192.168.1.50/nodes.json'"></label>
          <div><button class="secondary" type="button" @click="resetSubscriptionForm">取消</button><button class="primary" type="submit" :disabled="busy">保存</button></div>
        </form>
        <section class="subscription-list" aria-label="订阅列表">
          <div v-if="!subscriptionList.length" class="proxy-empty">尚未添加订阅。</div>
          <article v-for="item in subscriptionList" :key="item.id"><div><strong>{{ item.name }}</strong><small>{{ item.host }} · {{ item.node_count }} 个节点</small><span v-if="item.last_error" class="test-failed">{{ item.last_error }}</span></div><div class="proxy-actions"><button class="secondary" type="button" :disabled="refreshingSubscriptionID === item.id" @click="refreshSubscription(item)">{{ refreshingSubscriptionID === item.id ? '更新中' : '更新' }}</button><button class="secondary" type="button" @click="editSubscription(item)">编辑</button><button class="delete-button" type="button" @click="deleteSubscription(item)">删除</button></div></article>
        </section>
      </template>

      <template v-else-if="view === 'diagnostics'">
         <section class="diagnostic-assessment" aria-label="diagnostic assessment">
           <div class="assessment-heading"><div><p class="section-kicker">诊断结果</p><h2>问题与修复建议</h2></div><small>建议只提供安全的下一步，不会自动修改网卡或绕过授权。</small></div>
           <article v-for="item in diagnosticRecommendations" :key="`${item.title}-${item.detail}`" :class="['recommendation', item.severity]"><div><strong>{{ item.title }}</strong><span>{{ item.detail }}</span></div><button v-if="item.action !== 'none'" class="secondary" type="button" @click="applyRecommendation(item.action)">{{ recommendationActionText(item.action) }}</button></article>
         </section>
         <section class="probe-panel">
          <div><p class="section-kicker">主动健康探测</p><h2>验证出口路径</h2></div>
          <label>协议<select v-model="probeProtocol"><option value="tcp">TCP</option><option value="udp">UDP</option><option value="dns">DNS</option></select></label>
          <label>预期出口<select v-model="probeRole"><option value="A">出口 A</option><option value="B">出口 B</option></select></label>
          <label class="probe-target">目标<input v-model="probeTarget" :placeholder="probeProtocol === 'dns' ? '223.5.5.5:53' : '1.1.1.1:443'"></label>
          <label v-if="probeProtocol === 'dns'" class="probe-target">查询名称<input v-model="probeDNSName"></label>
          <button class="secondary" type="button" :disabled="busy || !probeTarget" @click="runProbe">运行探测</button>
        </section>
        <section class="observation-strip" aria-label="诊断摘要">
          <div><span>探测记录</span><strong>{{ observations.probes.length }}</strong><small>{{ observations.probes.at(-1)?.success ? '最近一次通过' : observations.probes.length ? '最近一次失败' : '尚未执行' }}</small></div>
          <div><span>接口计数器</span><strong>{{ observations.counters.length }}</strong><small>仅用于趋势辅助诊断</small></div>
          <div><span>Rule-set</span><strong>{{ observations.rule_sets.length }}</strong><small>{{ observations.rule_sets[0]?.load_result ?? '核心应用后记录' }}</small></div>
        </section>
        <section class="log-toolbar"><div><p class="section-kicker">运行事件</p><h2>最近 200 条</h2></div><label>日志级别<select v-model="logFilter"><option value="all">全部</option><option value="info">信息</option><option value="warning">警告</option><option value="error">错误</option></select></label></section>
        <section class="log-list" aria-live="polite" aria-label="实时日志">
          <div v-if="!filteredLogs.length" class="log-empty">当前筛选条件下没有日志。</div>
          <article v-for="entry in filteredLogs" :key="entry.id" class="log-entry"><time>{{ entry.time }}</time><span :class="['log-level', entry.level]">{{ levelText(entry.level) }}</span><p>{{ entry.message }}</p><code v-if="entry.correlation">{{ entry.correlation }}</code></article>
        </section>
        <section class="export-band">
          <div><p class="section-kicker">诊断包</p><h2>脱敏预览与导出</h2><p>完整 rule-set 二进制不会包含在导出内容中。</p></div>
          <div class="export-actions"><button class="secondary" type="button" @click="previewDiagnostics">预览</button><button class="primary" type="button" :disabled="busy || !diagnosticPreview" @click="exportDiagnostics">导出 ZIP</button></div>
        </section>
        <section v-if="diagnosticPreview" class="bundle-preview"><strong>{{ diagnosticPreview.files.length }} 个文件 · {{ diagnosticPreview.log_count }} 条日志 · {{ diagnosticPreview.probe_count }} 条探测</strong><span>{{ diagnosticPreview.files.join('、') }}</span><small v-for="note in diagnosticPreview.sensitive_notes" :key="note">{{ note }}</small></section>
      </template>

      <template v-else>
        <section class="settings-list" aria-label="IPv6 分流设置"><div class="setting-row"><div><h2>IPv6 分流</h2><p>关闭时拒绝 AAAA 查询和 IPv6 流量；开启前必须通过两个出口的 IPv6 地址、默认路由与前缀重叠预检。</p></div><div class="budget-controls"><label class="toggle"><input v-model="ipv6Policy" type="checkbox" true-value="split" false-value="block" :disabled="ipv6Busy || isRunning"><span>{{ ipv6Policy === 'split' ? '已启用' : '已关闭' }}</span></label><button type="button" class="primary" :disabled="ipv6Busy || isRunning" @click="saveIPv6Policy">{{ ipv6Busy ? '正在预检' : '保存 IPv6 策略' }}</button></div></div></section>
        <section class="settings-list" :aria-label="t('settings.language')">
          <div class="setting-row">
            <div><h2>{{ t('settings.language') }}</h2><p>{{ t('settings.languageDetail') }}</p></div>
            <select class="language-select" :value="locale" @change="changeLocale"><option value="zh-CN">简体中文</option><option value="en-US">English</option></select>
          </div>
        </section>
        <section class="settings-list" aria-label="流量预算设置">
          <div class="setting-row budget-setting">
            <div><h2>{{ t('settings.budget') }}</h2><p>{{ t('settings.budgetDetail') }}</p></div>
            <div class="budget-controls">
              <label class="toggle"><input v-model="budgetForm.enabled" type="checkbox" :disabled="budgetBusy"><span>{{ t(budgetForm.enabled ? 'common.enabled' : 'common.disabled') }}</span></label>
              <label>{{ t('settings.budgetGB') }}<input v-model.number="budgetForm.budget_gb" type="number" min="0.1" max="100000" step="0.1" :disabled="budgetBusy"></label>
              <label>{{ t('settings.warning') }}<input v-model.number="budgetForm.warning_percent" type="number" min="1" max="100" step="1" :disabled="budgetBusy"></label>
              <button type="button" class="primary" :disabled="budgetBusy" @click="saveTrafficBudget">{{ t('common.save') }}</button>
              <button type="button" class="secondary" :disabled="budgetBusy || !observations.traffic_budget.used_bytes" @click="resetTrafficBudget">{{ t('settings.resetMonth') }}</button>
            </div>
          </div>
        </section>
        <section class="settings-list" aria-label="应用设置">
          <div class="setting-row">
            <div><h2>{{ t('settings.autostart') }}</h2><p>{{ t('settings.autostartDetail') }}</p></div>
            <label class="toggle"><input v-model="autostartEnabled" type="checkbox" :disabled="autostartBusy" @change="updateAutostart"><span>{{ t(autostartEnabled ? 'common.enabled' : 'common.disabled') }}</span></label>
          </div>
        </section>
        <section class="settings-list network-reset-section" aria-label="应用故障恢复">
          <div class="setting-row network-reset-setting">
            <div><p class="section-kicker">故障恢复</p><h2>应用配置恢复</h2><p>仅重置网卡选择，或在自动备份后恢复规则、DNS、IPv6 策略和网卡选择默认值。不会删除代理节点、订阅、规则集来源，也不会修改 Windows 网络组件。</p><div class="config-directory"><span>当前用户配置目录</span><code>{{ applicationConfigDirectory || '无法确定配置目录' }}</code><small>卸载程序不会自动删除此目录；需要完整移除 WinRouter 时可手动删除。</small></div></div>
            <div class="proxy-actions"><button type="button" class="secondary" :disabled="applicationResetBusy" @click="resetSavedInterfaces">重置网卡选择</button><button type="button" class="secondary" :disabled="applicationResetBusy" @click="resetApplicationSettings">恢复应用默认设置</button><button type="button" class="danger-button" :disabled="applicationResetBusy" @click="repairApplicationSettings">{{ applicationResetBusy ? '正在处理' : '强制修复配置' }}</button></div>
          </div>
          <div class="setting-row"><div><h2>配置文件位置</h2><p>规则、网卡选择、DNS、代理节点、订阅、缓存和恢复备份均保存在当前用户目录，不位于程序安装目录。</p></div><button type="button" class="secondary" :disabled="!applicationConfigDirectory" @click="copyApplicationConfigDirectory">复制路径</button></div>
        </section>
        <section class="settings-list network-reset-section" aria-label="高级网络修复">
          <div class="setting-row network-reset-setting">
            <div><p class="section-kicker">高级修复</p><h2>重置 Windows 网络组件</h2><p class="danger-copy">此操作会停止分流核心，并重置 Winsock、TCP/IP 与 DNS 缓存。可能清除静态 IP、网关、DNS、VPN 或虚拟网卡配置，导致网络中断；完成后通常需要重启 Windows。</p></div>
            <button type="button" class="danger-button" :disabled="networkResetBusy" @click="resetWindowsNetworkStack">{{ networkResetBusy ? '正在重置' : '重置网络组件' }}</button>
          </div>
          <div v-if="networkResetResult" class="network-reset-result" :class="{ failed: !networkResetResult.success }">
            <strong>{{ networkResetResult.success ? '全部步骤已完成' : '部分步骤执行失败' }} · {{ networkResetResult.restart_required ? '需要重启 Windows' : '无需重启' }}</strong>
            <div v-for="step in networkResetResult.steps" :key="step.command" class="network-reset-step">
              <span :class="step.success ? 'step-success' : 'step-failed'">{{ step.success ? '成功' : `失败 (${step.exit_code})` }}</span><code>{{ step.command }}</code>
              <pre v-if="step.output">{{ step.output }}</pre>
            </div>
          </div>
        </section>
      </template>
    </main>
  </div>
</template>
