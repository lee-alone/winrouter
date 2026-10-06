import { ref } from 'vue'
import {
  SpeedTestProxyNodes,
  TestProxyChain,
  TestProxyNode,
} from '../../wailsjs/go/app/App'
import type { nodes } from '../types'
import { getBootstrapDNSServer } from './useDNSManager'
import { error, messageOf, notice } from './useFeedback'
import { proxySelection } from './useProxyChain'

export const proxyTests = ref<Record<string, nodes.TestResult>>({})
export const testingProxyID = ref('')
export const testingAllProxies = ref(false)
export const chainTestResult = ref<nodes.TestResult | null>(null)
export const testingChain = ref(false)

export function formatErrorCategory(cat?: string): string {
  switch (cat) {
    case 'dns_failed': return 'DNS 解析失败'
    case 'egress_unavailable': return '出口网卡不可用'
    case 'tcp_failed': return 'TCP 连接失败'
    case 'tls_failed': return 'TLS 握手失败'
    case 'authentication_failed': return '凭据认证失败'
    case 'protocol_failed': return '协议通信失败'
    case 'target_failed': return '测试目标异常'
    case 'timeout': return '请求超时'
    case 'core_failed': return '临时核心异常'
    default: return '测试失败'
  }
}

export async function testProxy(node: nodes.Node): Promise<void> {
  testingProxyID.value = node.id
  error.value = ''
  try {
    const result = await TestProxyNode(node.id, getBootstrapDNSServer())
    proxyTests.value = { ...proxyTests.value, [node.id]: result }
  } catch (reason) {
    error.value = `节点测试失败：${messageOf(reason)}`
  } finally {
    testingProxyID.value = ''
  }
}

export async function speedTestAllProxies(): Promise<void> {
  testingAllProxies.value = true
  error.value = ''
  try {
    const results = await SpeedTestProxyNodes(getBootstrapDNSServer())
    const next = { ...proxyTests.value }
    for (const result of results) next[result.node_id] = result
    proxyTests.value = next
    notice.value = `测速完成：${results.filter(item => item.available).length}/${results.length} 个节点可用。`
  } catch (reason) {
    error.value = `批量测速失败：${messageOf(reason)}`
  } finally {
    testingAllProxies.value = false
  }
}

export async function testChain(): Promise<void> {
  const curChain = Array.isArray(proxySelection.value?.selected_chain) ? proxySelection.value.selected_chain : []
  if (curChain.length < 2) {
    error.value = '套接链至少需要 2 个节点才能进行整链测试。'
    return
  }
  testingChain.value = true
  error.value = ''
  try {
    const result = await TestProxyChain(curChain, getBootstrapDNSServer())
    chainTestResult.value = result
    if (result.available) {
      notice.value = `整链测试成功！延迟 ${result.latency_ms} ms。`
    } else {
      error.value = `整链测试不可用：${formatErrorCategory(result.error_category)}${result.error ? ` (${result.error})` : ''}`
    }
  } catch (reason) {
    error.value = `整链测试失败：${messageOf(reason)}`
  } finally {
    testingChain.value = false
  }
}
