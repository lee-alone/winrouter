import type { app as appModels, autostart, configdir, core } from '../../wailsjs/go/models'

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

// Domain aliases for Wails models
export type ApplicationStatus = appModels.ApplicationStatus
export type NetworkResetStep = appModels.NetworkResetStep
export type NetworkResetResult = appModels.NetworkResetResult
export type CoreStatus = core.Status
export type AutostartStatus = autostart.Status
export type ConfigDirInfo = configdir.Info

// Re-export underlying namespaces for seamless compatibility
export type { appModels as app, core, autostart, configdir }
