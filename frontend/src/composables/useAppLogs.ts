import { computed, ref } from 'vue'
import { RecordApplicationLog } from '../../wailsjs/go/app/App'
import { t } from '../i18n'
import type { LogEntry, LogLevel } from '../types'

export const logs = ref<LogEntry[]>([])
export const logFilter = ref<'all' | LogLevel>('all')

let nextLogID = 1

export function addLog(level: LogLevel, message: string, correlated = false) {
  const correlation = correlated ? `WR-${Date.now().toString(36).toUpperCase()}-${nextLogID}` : undefined
  logs.value.unshift({
    id: nextLogID++,
    time: new Date().toLocaleTimeString('zh-CN', { hour12: false }),
    level,
    message,
    correlation,
  })
  logs.value = logs.value.slice(0, 200)
  void RecordApplicationLog(level, message, correlation ?? '')
}

export const filteredLogs = computed(() =>
  logFilter.value === 'all' ? logs.value : logs.value.filter(entry => entry.level === logFilter.value)
)

export function levelText(level: LogLevel) {
  return t(`log.${level}`)
}
