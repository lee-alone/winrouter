/**
 * useRulesManager - 规则管理模块统一入口（门面层）
 * 聚合拆分后的各子模块，保持既有引用向后兼容：
 * - useCustomRules: 普通规则列表、表单状态、增删改与主规则排序
 * - useSRSRules: SRS 规则集的拉取、刷新、预设选择与更新时间格式化
 * - useRulePreview: 规则编译预览及进程规则状态检查
 */

export * from './useRuleProfiles'
export * from './useCustomRules'
export * from './useSRSRules'
export * from './useRulePreview'
