import type { app as appModels, config, core, interfacemanager, interfaces, nodes, observability, processrules, rulesettings, srssets, subscriptions } from '../../wailsjs/go/models'

export type View = 'overview' | 'monitoring' | 'interfaces' | 'rules' | 'proxy' | 'diagnostics' | 'settings'

export type LogLevel = 'info' | 'warning' | 'error'

export interface LogEntry {
  id: number
  time: string
  level: LogLevel
  message: string
  correlation?: string
}

export interface RecoveryStatus {
  state: 'idle' | 'monitoring' | 'stopping' | 'waiting-for-network' | 'recovering' | 'failed'
  desired_running: boolean
  attempts: number
  last_error?: string
  last_change?: string
  snapshot_sequence?: number
}

export interface ConnectionSummary {
  active_tcp: number
  established_tcp: number
  listening_tcp: number
  udp_endpoints: number
  sampled_at?: string
}

export interface RuleHit {
  outbound: string
  count: number
}

export interface ConnectionEvent {
  domain?: string
  address_type?: string
  ip?: string
  protocol?: string
  rule?: string
  outbound?: string
  attempts: number
  established: number
  bytes_up: number
  bytes_down: number
  last_seen?: string
}

export interface TrafficPoint {
  at: number
  aDown: number
  aUp: number
  bDown: number
  bUp: number
}

export interface CounterBaseline {
  at: number
  received: number
  transmitted: number
}

export interface UsageBaseline {
  received: number
  transmitted: number
  reset_at: string
}

export interface DiagnosticRecommendation {
  severity: 'error' | 'warning' | 'info'
  title: string
  detail: string
  action: 'interfaces' | 'probe' | 'export' | 'none'
}

export interface TrafficBudgetStatus {
  enabled: boolean
  budget_gb: number
  warning_percent: number
  period: string
  used_bytes: number
  budget_bytes: number
  used_percent: number
  warning_reached: boolean
  limit_reached: boolean
  interface_guid?: string
}

export interface DNSServer {
  preset_id?: string
  type: 'udp' | 'tls' | 'https'
  server: string
  port: number
  server_name?: string
}

export interface DNSSettings {
  schema_version: number
  domestic: DNSServer
  global: DNSServer
  proxy?: DNSServer
}

export type DNSPreset = DNSServer & {
  id: string
  name: string
  scope: 'domestic' | 'global' | 'proxy'
}

export type DNSTestState = 'idle' | 'testing' | 'success' | 'failed'

export interface ProxyProtocolMeta {
  value: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  label: string
  authKind: 'user-pass' | 'ss' | 'uuid' | 'trojan'
  supportsTLS: boolean
  forceTLS: boolean
  supportsTransport: boolean
  supportsFlow: boolean
  allowNoSecret: boolean
}

export type InlineRuleType = 'domain-suffix' | 'domain' | 'ip' | 'process-name' | 'process-path'

export type RuleAction = 'a' | 'b' | 'c' | 'final' | 'reject'

export interface CustomRule {
  id: string
  name: string
  type: InlineRuleType
  values: string[];
  action: RuleAction
  enabled: boolean
}

export interface RuleForm {
  id: string
  name: string
  type: InlineRuleType | 'rule-set'
  valuesText: string
  action: RuleAction
  enabled: boolean
}

export type MasterRow =
  | { key: string; kind: 'custom'; rule: CustomRule }
  | { key: string; kind: 'srs'; source: srssets.Source }
