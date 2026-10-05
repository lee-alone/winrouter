/**
 * useProxyManager - 代理管理模块统一入口（门面层）
 * 聚合拆分后的各子模块，保持既有引用向后兼容：
 * - useProxyNodes: 节点定义、状态列表、排序、默认选择与核心操作
 * - useProxyNodeForm: 节点表单状态、协议字段、输入校验、持久化保存与链接导入
 * - useSubscriptions: 订阅列表、更新校验与订阅表单
 * - useProxyChain: 链式套接编排、跳板顺序与流水线状态
 * - useProxyProbe: 节点单测、全量批量测速与整链延迟探测
 */

export * from './useProxyNodes'
export * from './useProxyNodeForm'
export * from './useSubscriptions'
export * from './useProxyChain'
export * from './useProxyProbe'
