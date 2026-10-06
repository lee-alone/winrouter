import type { config, processrules, rulesettings, srssets } from '../../wailsjs/go/models'

export type InlineRuleType = 'domain-suffix' | 'domain' | 'ip' | 'process-name' | 'process-path'

export type RuleAction = 'a' | 'b' | 'c' | 'final' | 'reject'

export interface CustomRule {
  id: string
  name: string
  type: InlineRuleType
  values: string[]
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

// Domain aliases for Wails routing and rule models
export type SRSSource = srssets.Source
export type SRSPreset = srssets.Preset
export type RuleProfile = rulesettings.Profile
export type RuleSettingsRule = rulesettings.Rule
export type ProcessRuleStatus = processrules.Status
export type MVPConfig = config.MVPConfig
export type RulePreview = config.RulePreview

// Re-export underlying namespaces for seamless compatibility
export type { config, processrules, rulesettings, srssets }
