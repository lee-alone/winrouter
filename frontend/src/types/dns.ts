import type { dnssettings } from '../../wailsjs/go/models'

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

// Domain aliases for Wails DNS models
export type DNSSettingsModel = dnssettings.Settings
export type DNSPresetModel = dnssettings.Preset

// Re-export underlying namespace for seamless compatibility
export type { dnssettings }
