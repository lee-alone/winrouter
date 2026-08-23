import { ref } from 'vue'

export type Locale = 'zh-CN' | 'en-US'

const storageKey = 'winrouter.locale.v1'
const fallbackLocale: Locale = 'zh-CN'

const messages: Record<Locale, Record<string, string>> = {
  'zh-CN': {
    'language.name': '简体中文',
    'nav.label': '主导航', 'nav.overview': '概览', 'nav.monitoring': '监控', 'nav.interfaces': '网卡', 'nav.rules': '规则', 'nav.proxy': '代理', 'nav.diagnostics': '诊断', 'nav.settings': '设置',
    'mode.direct': '多出口策略路由', 'mode.proxy': '严格代理分流',
    'title.overview': '运行概览', 'title.monitoring': '实时监控', 'title.interfacesFirst': '首次配置', 'title.interfaces': '网卡设置', 'title.rules': '自定义规则', 'title.proxy': '代理节点', 'title.diagnostics': '诊断日志', 'title.settings': '应用设置',
    'common.enabled': '已启用', 'common.disabled': '未启用', 'common.save': '保存', 'common.none': '无', 'common.closeError': '关闭错误', 'common.closeNotice': '关闭提示',
    'banner.failed': '操作未完成', 'banner.success': '操作成功',
    'state.processing': '正在处理', 'state.waitingNetwork': '等待网络恢复', 'state.stopping': '正在安全停止', 'state.recovering': '正在恢复', 'state.recoveryFailed': '恢复失败', 'state.needsRecovery': '需要恢复', 'state.running': '运行中', 'state.ready': '已就绪', 'state.unconfigured': '尚未配置',
    'overview.selection': '用户选择', 'overview.completed': '已完成', 'overview.pending': '待配置', 'overview.core': '核心应用', 'overview.applied': '已应用', 'overview.notApplied': '未应用', 'overview.networkUnmanaged': '系统网络未接管', 'overview.verification': '出口验证', 'overview.healthy': '核心健康', 'overview.unverified': '未验证', 'overview.ipv6': 'IPv6 策略', 'overview.blocked': '已阻止', 'overview.leakProtection': '避免未分流流量泄漏',
    'overview.getStarted': '开始使用', 'overview.chooseOutlets': '选择两个独立的网络出口', 'overview.outletDescription': '出口 A 承载国内/局域网流量，出口 B 承载公网直连流量，出口 C 承载代理流量。', 'overview.configure': '配置网卡', 'overview.runMode': '默认出口策略', 'overview.directRoute': '未匹配流量默认走出口 B', 'overview.proxyRoute': '未匹配流量默认走出口 C (代理)', 'overview.direct': '直连', 'overview.proxy': '代理', 'overview.coreControl': '核心控制', 'overview.running': '分流正在运行', 'overview.interrupted': '核心异常中断', 'overview.stopped': '分流已停止', 'overview.runningDetail': '配置已应用，系统流量正在按多出口策略路由转发。', 'overview.stoppedDetail': '启动前将执行模型、Schema、语义和核心四层校验。', 'overview.starting': '正在启动', 'overview.start': '启动', 'overview.restart': '重新启动', 'overview.stop': '停止', 'overview.domestic': '国内与局域网', 'overview.public': '公网直连', 'overview.status': '状态', 'overview.address': '地址', 'overview.gateway': '网关',
    'monitor.activeTcp': '活动 TCP', 'monitor.established': '已建立', 'monitor.listening': '监听', 'monitor.udp': 'UDP 端点', 'monitor.systemSnapshot': 'Windows IPv4 系统快照', 'monitor.hitA': '出口 A 命中', 'monitor.hitB': '出口 B 命中', 'monitor.logWindow': '核心保留日志窗口', 'monitor.budget': '网卡 B 月度预算', 'monitor.budgetScope': '统计网卡 B 的全部收发流量，包含其他应用；每个自然月自动归零。', 'monitor.interfaceTraffic': '接口流量', 'monitor.recent': '最近 2.5 分钟', 'monitor.sampling': '5 秒采样 · 窗口内自动缩放', 'monitor.download': '下载', 'monitor.upload': '上传', 'monitor.baseline': '正在建立采样基线', 'monitor.earlier': '较早', 'monitor.now': '现在', 'monitor.ruleScope': '规则命中范围', 'monitor.ruleScopeDetail': '仅统计当前核心保留日志中已确认的 outbound 连接记录，核心重启或日志滚动后重新计数。', 'monitor.connectionScope': '连接范围', 'monitor.connectionScopeDetail': '连接数来自 Windows IPv4 TCP/UDP 系统表，用于判断总体负载，不表示单个应用归属。',
    'settings.language': '界面语言', 'settings.languageDetail': '语言选择仅保存在当前用户浏览器配置中，切换后立即生效。', 'settings.budget': '网卡 B 月度流量预算', 'settings.budgetDetail': '达到阈值时提醒，不会自动断网。统计包含该网卡上的其他应用流量，不等同于运营商账单。', 'settings.budgetGB': '预算 GB', 'settings.warning': '提醒比例 %', 'settings.resetMonth': '本月归零', 'settings.autostart': '开机启动', 'settings.autostartDetail': '登录当前 Windows 用户后自动打开 WinRouter。分流核心仍需由用户启动并确认管理员授权。',
    'status.up': '已连接', 'status.down': '未连接', 'log.info': '信息', 'log.warning': '警告', 'log.error': '错误',
  },
  'en-US': {
    'language.name': 'English',
    'nav.label': 'Primary navigation', 'nav.overview': 'Overview', 'nav.monitoring': 'Monitoring', 'nav.interfaces': 'Interfaces', 'nav.rules': 'Rules', 'nav.proxy': 'Proxy', 'nav.diagnostics': 'Diagnostics', 'nav.settings': 'Settings',
    'mode.direct': 'Multi-Egress Policy Routing', 'mode.proxy': 'Strict proxy routing',
    'title.overview': 'Runtime overview', 'title.monitoring': 'Live monitoring', 'title.interfacesFirst': 'Initial setup', 'title.interfaces': 'Interface settings', 'title.rules': 'Custom rules', 'title.proxy': 'Proxy nodes', 'title.diagnostics': 'Diagnostic logs', 'title.settings': 'Application settings',
    'common.enabled': 'Enabled', 'common.disabled': 'Disabled', 'common.save': 'Save', 'common.none': 'None', 'common.closeError': 'Dismiss error', 'common.closeNotice': 'Dismiss notice',
    'banner.failed': 'Action incomplete', 'banner.success': 'Action completed',
    'state.processing': 'Processing', 'state.waitingNetwork': 'Waiting for network', 'state.stopping': 'Stopping safely', 'state.recovering': 'Recovering', 'state.recoveryFailed': 'Recovery failed', 'state.needsRecovery': 'Recovery required', 'state.running': 'Running', 'state.ready': 'Ready', 'state.unconfigured': 'Not configured',
    'overview.selection': 'Interface selection', 'overview.completed': 'Complete', 'overview.pending': 'Setup required', 'overview.core': 'Core configuration', 'overview.applied': 'Applied', 'overview.notApplied': 'Not applied', 'overview.networkUnmanaged': 'System network is not managed', 'overview.verification': 'Route verification', 'overview.healthy': 'Core healthy', 'overview.unverified': 'Not verified', 'overview.ipv6': 'IPv6 policy', 'overview.blocked': 'Blocked', 'overview.leakProtection': 'Prevents unrouted traffic leaks',
    'overview.getStarted': 'Get started', 'overview.chooseOutlets': 'Choose two independent network interfaces', 'overview.outletDescription': 'Interface A carries domestic/LAN traffic, Interface B carries direct traffic, and Exit C carries proxy traffic.', 'overview.configure': 'Configure interfaces', 'overview.runMode': 'Default Route Policy', 'overview.directRoute': 'Unmatched traffic routes via Interface B', 'overview.proxyRoute': 'Unmatched traffic routes via Exit C (Proxy)', 'overview.direct': 'Direct', 'overview.proxy': 'Proxy', 'overview.coreControl': 'Core control', 'overview.running': 'Routing is active', 'overview.interrupted': 'Core stopped unexpectedly', 'overview.stopped': 'Routing is stopped', 'overview.runningDetail': 'The configuration is applied and system traffic follows the multi-egress policy routing.', 'overview.stoppedDetail': 'Model, schema, semantic, and core validation run before startup.', 'overview.starting': 'Starting', 'overview.start': 'Start', 'overview.restart': 'Restart', 'overview.stop': 'Stop', 'overview.domestic': 'Domestic and LAN', 'overview.public': 'Direct Public Traffic', 'overview.status': 'Status', 'overview.address': 'Address', 'overview.gateway': 'Gateway',
    'monitor.activeTcp': 'Active TCP', 'monitor.established': 'established', 'monitor.listening': 'listening', 'monitor.udp': 'UDP endpoints', 'monitor.systemSnapshot': 'Windows IPv4 system snapshot', 'monitor.hitA': 'Interface A hits', 'monitor.hitB': 'Interface B hits', 'monitor.logWindow': 'Retained core log window', 'monitor.budget': 'Monthly interface B budget', 'monitor.budgetScope': 'Counts all traffic on interface B, including other applications; resets each calendar month.', 'monitor.interfaceTraffic': 'Interface traffic', 'monitor.recent': 'Last 2.5 minutes', 'monitor.sampling': '5-second samples · auto-scaled window', 'monitor.download': 'download', 'monitor.upload': 'upload', 'monitor.baseline': 'Establishing sampling baseline', 'monitor.earlier': 'Earlier', 'monitor.now': 'Now', 'monitor.ruleScope': 'Rule-hit scope', 'monitor.ruleScopeDetail': 'Counts confirmed outbound connection records in the retained core log. Counts restart after core restarts or log rotation.', 'monitor.connectionScope': 'Connection scope', 'monitor.connectionScopeDetail': 'Connection totals come from Windows IPv4 TCP/UDP tables and indicate overall load, not individual application ownership.',
    'settings.language': 'Display language', 'settings.languageDetail': 'The language choice is stored for the current user and takes effect immediately.', 'settings.budget': 'Monthly interface B traffic budget', 'settings.budgetDetail': 'Warns at the threshold without disconnecting the network. Counts other applications on this interface and may differ from carrier billing.', 'settings.budgetGB': 'Budget (GB)', 'settings.warning': 'Warning %', 'settings.resetMonth': 'Reset month', 'settings.autostart': 'Launch at sign-in', 'settings.autostartDetail': 'Opens WinRouter after the current Windows user signs in. Starting the routing core still requires explicit user action and administrator approval.',
    'status.up': 'Connected', 'status.down': 'Disconnected', 'log.info': 'Info', 'log.warning': 'Warning', 'log.error': 'Error',
  },
}

function validateMessages() {
  const required = Object.keys(messages[fallbackLocale])
  for (const [name, catalog] of Object.entries(messages)) {
    const missing = required.filter(key => !catalog[key])
    if (missing.length) throw new Error(`Missing ${name} translations: ${missing.join(', ')}`)
  }
}

function initialLocale(): Locale {
  const stored = localStorage.getItem(storageKey)
  if (stored === 'zh-CN' || stored === 'en-US') return stored
  return navigator.language.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

export const locale = ref<Locale>(initialLocale())

export function setLocale(next: Locale) {
  locale.value = next
  localStorage.setItem(storageKey, next)
  document.documentElement.lang = next
}

export function t(key: string, values: Record<string, string | number> = {}): string {
  const template = messages[locale.value][key] ?? messages[fallbackLocale][key] ?? key
  return template.replace(/\{(\w+)\}/g, (_, name: string) => String(values[name] ?? `{${name}}`))
}

validateMessages()
setLocale(locale.value)
