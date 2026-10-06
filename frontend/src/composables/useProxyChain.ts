import { computed, ref } from 'vue'
import {
  SelectProxyChain,
  SetProxyMode,
} from '../../wailsjs/go/app/App'
import type { nodes } from '../types'
import { error, messageOf, notice } from './useFeedback'
import { proxyNodes, refreshProxyState } from './useProxyNodes'
import { chainTestResult } from './useProxyProbe'

export const proxySelection = ref<nodes.ProxySelection>({ mode: 'single', selected_id: '', selected_chain: [] })

export const isChainMode = computed(() => proxySelection.value?.mode === 'chain' && (proxyNodes.value?.length || 0) > 0)

export const chainNodes = computed(() => {
  const list = proxyNodes.value || []
  const map = new Map(list.map(n => [n.id, n]))
  const chainIds = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  return chainIds.map(id => map.get(id)).filter((n): n is nodes.Node => Boolean(n))
})

export const isChainReady = computed(() => isChainMode.value && chainNodes.value.length >= 2)

export const chainSummaryText = computed(() => {
  if (!chainNodes.value || !chainNodes.value.length) return '未配置链路'
  return chainNodes.value.map(n => n?.name || '未知节点').join(' ➔ ')
})

export function applyProxySelection(sel: nodes.ProxySelection | null | undefined, currentNodes: nodes.Node[]): void {
  const selChain = Array.isArray(sel?.selected_chain) ? sel.selected_chain : []
  const validChain = selChain.filter(id => currentNodes.some(n => n.id === id))
  const rawMode = sel?.mode || 'single'
  const mode = rawMode === 'chain' && currentNodes.length > 0 ? 'chain' : 'single'
  proxySelection.value = {
    mode,
    selected_id: sel?.selected_id || '',
    selected_chain: validChain,
  }
}

export async function switchProxyMode(mode: 'single' | 'chain', isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止切换代理模式；请先停止核心。'
    return
  }
  if (mode === 'chain' && (!proxyNodes.value || proxyNodes.value.length === 0)) {
    notice.value = '尚未添加代理节点，请先添加或导入节点后再启用链式套接模式。'
    return
  }
  error.value = ''
  try {
    await SetProxyMode(mode)
    await refreshProxyState()
    notice.value = mode === 'chain' ? '已切换至链式套接模式。' : '已切换至单节点直连模式。'
  } catch (reason) {
    error.value = `切换代理模式失败：${messageOf(reason)}`
  }
}

export function isNodeInChain(id: string): boolean {
  return Array.isArray(proxySelection.value?.selected_chain) && proxySelection.value.selected_chain.includes(id)
}

export function chainNodeHopIndex(id: string): number {
  return Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain.indexOf(id) : -1
}

export async function addToChain(node: nodes.Node, isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止修改代理链路；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (curChain.includes(node.id)) return
  const nextChain = [...curChain, node.id]
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
    notice.value = `已将“${node.name}”添加到代理链路末端。`
  } catch (reason) {
    error.value = `添加链路节点失败：${messageOf(reason)}`
  }
}

export async function removeFromChain(index: number, isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止修改代理链路；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (index < 0 || index >= curChain.length) return
  const nextChain = [...curChain]
  const removedId = nextChain.splice(index, 1)[0]
  const nodeName = proxyNodes.value.find(n => n.id === removedId)?.name || '节点'
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
    notice.value = `已从链路中移除“${nodeName}”。`
  } catch (reason) {
    error.value = `移除链路节点失败：${messageOf(reason)}`
  }
}

export async function moveChainHop(index: number, direction: 'up' | 'down', isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止调整链路顺序；请先停止核心。'
    return
  }
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  const targetIndex = direction === 'up' ? index - 1 : index + 1
  if (targetIndex < 0 || targetIndex >= curChain.length || index < 0 || index >= curChain.length) return
  const nextChain = [...curChain]
  const [moved] = nextChain.splice(index, 1)
  nextChain.splice(targetIndex, 0, moved)
  try {
    await SelectProxyChain(nextChain)
    await refreshProxyState()
  } catch (reason) {
    error.value = `调整链路顺序失败：${messageOf(reason)}`
  }
}

export async function clearChain(isRunning = false): Promise<void> {
  if (isRunning) {
    error.value = '核心运行中，禁止清空代理链路；请先停止核心。'
    return
  }
  try {
    await SelectProxyChain([])
    await refreshProxyState()
    chainTestResult.value = null
    notice.value = '已清空代理链路。'
  } catch (reason) {
    error.value = `清空代理链路失败：${messageOf(reason)}`
  }
}
