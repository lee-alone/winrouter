import type { interfacemanager, interfaces, trafficbudget } from '../../wailsjs/go/models'

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

// Domain aliases for Wails network models
export type Adapter = interfaces.Adapter
export type InterfaceSnapshot = interfacemanager.Snapshot
export type ResolvedInterfaceSelection = interfacemanager.ResolvedSelection
export type InterfaceCandidate = interfacemanager.Candidate
export type TrafficBudgetModel = trafficbudget.Status

// Re-export underlying namespaces for seamless compatibility
export type { interfaces, interfacemanager, trafficbudget }
