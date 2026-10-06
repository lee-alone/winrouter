import type { clashapi, observability } from '../../wailsjs/go/models'
import type { TrafficBudgetStatus } from './network'

export interface ConnectionSummary {
  active_tcp: number
  established_tcp: number
  listening_tcp: number
  udp_endpoints: number
  sampled_at?: string
}

export interface RealtimeMetrics {
  counters: observability.InterfaceCounter[]
  connections: ConnectionSummary
  traffic_budget: TrafficBudgetStatus
  connection_observation: boolean
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

export interface DiagnosticRecommendation {
  severity: 'error' | 'warning' | 'info'
  title: string
  detail: string
  action: 'interfaces' | 'probe' | 'export' | 'none'
}

// Domain aliases for Wails observability models
export type ProbeResult = observability.ProbeResult
export type InterfaceCounter = observability.InterfaceCounter
export type RuleSetMetadata = observability.RuleSetMetadata
export type DiagnosticBundlePreview = observability.BundlePreview
export type ConnectionEventSummary = clashapi.Summary

// Re-export underlying namespaces for seamless compatibility
export type { observability, clashapi }
