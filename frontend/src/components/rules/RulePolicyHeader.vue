<script setup lang="ts">
import { isRunning } from '../../composables/useCoreManager'
import { dnsSettings } from '../../composables/useDNSManager'
import {
  selectedAdapterA,
  selectedAdapterB,
} from '../../composables/useNetworkInterfaces'
import {
  chainSummaryText,
  isChainMode,
  isChainReady,
  proxyNodes,
} from '../../composables/useProxyManager'
import {
  defaultOutbound,
  ruleUpdateOutbound,
  selectedRuleMode,
  updateDefaultOutbound,
  updateRuleUpdateOutbound,
} from '../../composables/useRuleProfiles'
</script>

<template>
  <section class="rule-outlet-map" aria-label="规则出口映射">
    <article>
      <span>{{ selectedRuleMode === 'single' ? '物理主网卡' : '网卡 A' }}</span>
      <strong>{{ selectedAdapterA?.friendly_name ?? '尚未选择' }}</strong>
      <small>{{ selectedAdapterA?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterA?.gateways?.[0] ?? '无网关' }}</small>
    </article>
    <article v-if="selectedRuleMode === 'dual'">
      <span>网卡 B</span>
      <strong>{{ selectedAdapterB?.friendly_name ?? '尚未选择' }}</strong>
      <small>{{ selectedAdapterB?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterB?.gateways?.[0] ?? '无网关' }}</small>
    </article>
  </section>

  <section class="policy-strip" aria-label="当前 DNS 出口">
    <div>
      <span>{{ selectedAdapterA?.friendly_name ?? (selectedRuleMode === 'single' ? '主网卡' : '网卡 A') }} DNS</span>
      <strong>{{ dnsSettings.domestic.type.toUpperCase() }} · {{ dnsSettings.domestic.server }}:{{ dnsSettings.domestic.port }}</strong>
    </div>
    <div v-if="selectedRuleMode === 'dual'">
      <span>{{ selectedAdapterB?.friendly_name ?? '网卡 B' }} DNS</span>
      <strong>{{ dnsSettings.global.type.toUpperCase() }} · {{ dnsSettings.global.server }}:{{ dnsSettings.global.port }}</strong>
    </div>
    <div>
      <span>出口 C (代理) DNS</span>
      <strong>{{ (dnsSettings.proxy?.type ?? dnsSettings.global.type).toUpperCase() }} · {{ dnsSettings.proxy?.server ?? dnsSettings.global.server }}:{{ dnsSettings.proxy?.port ?? dnsSettings.global.port }}</strong>
    </div>
  </section>

  <section class="fallback-outbound" aria-label="未匹配流量兜底出口">
    <div>
      <strong>未匹配流量兜底出口</strong>
      <small>未命中自定义规则的流量将从此出口发出。</small>
    </div>
    <select
      :value="defaultOutbound"
      :disabled="isRunning"
      @change="updateDefaultOutbound(($event.target as HTMLSelectElement).value as 'a' | 'b' | 'c')"
    >
      <option value="a">
        {{ selectedRuleMode === 'single' ? '直连 (主网卡) · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '未选择' }}
      </option>
      <option v-if="selectedRuleMode === 'dual'" value="b">
        出口 B · {{ selectedAdapterB?.friendly_name ?? '未选择' }}
      </option>
      <option value="c">
        出口 C · 代理出站 ({{ isChainMode && isChainReady ? `套接: ${chainSummaryText}` : (proxyNodes.find(n => n.selected)?.name ?? '未配置') }})
      </option>
    </select>
  </section>

  <section class="fallback-outbound" aria-label="规则更新出口通道">
    <div>
      <strong>规则更新出口通道</strong>
      <small>拉取与更新 SRS 分流规则时使用的网络通道。</small>
    </div>
    <select
      :value="ruleUpdateOutbound"
      @change="updateRuleUpdateOutbound(($event.target as HTMLSelectElement).value as 'auto' | 'a' | 'b' | 'c')"
    >
      <option value="auto">自动选择 (代理就绪优先走代理，无代理走直连)</option>
      <option value="c">
        出口 C · 代理出站 ({{ isChainMode && isChainReady ? `套接: ${chainSummaryText}` : (proxyNodes.find(n => n.selected)?.name ?? '未配置') }})
      </option>
      <option v-if="selectedRuleMode === 'dual'" value="b">
        出口 B · {{ selectedAdapterB?.friendly_name ?? '网卡 B' }}
      </option>
      <option value="a">
        {{ selectedRuleMode === 'single' ? '主网卡直连 · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}
      </option>
    </select>
  </section>
</template>
