<script setup lang="ts">
import { isRunning } from '../../composables/useCoreManager'
import { routingMode } from '../../composables/useNetworkInterfaces'
import {
  chainNodes,
  clearChain,
  isChainMode,
  moveChainHop,
  removeFromChain,
} from '../../composables/useProxyChain'
import {
  chainTestResult,
  formatErrorCategory,
  testChain,
  testingChain,
} from '../../composables/useProxyProbe'
</script>

<template>
  <!-- Chain Pipeline Visualizer (Only visible in chain mode) -->
  <section v-if="isChainMode" class="chain-pipeline-card" aria-label="代理链流水线">
    <div class="chain-pipeline-header">
      <div>
        <span class="section-kicker">链路流水线设计</span>
        <h3>套接链路流水线 ({{ chainNodes.length }} 跳)</h3>
        <p class="proxy-form-hint">第一跳为前置中转（跳板，直连物理出口），最后一跳为落地节点（业务目标可见地址）。</p>
      </div>
      <div class="chain-pipeline-actions">
        <button
          class="primary"
          type="button"
          :disabled="testingChain || chainNodes.length < 2"
          @click="testChain"
        >
          {{ testingChain ? '整链测试中...' : '测试整链' }}
        </button>
        <button
          class="secondary"
          type="button"
          :disabled="isRunning || !chainNodes.length"
          @click="clearChain(isRunning)"
        >
          清空链路
        </button>
      </div>
    </div>

    <!-- Chain Pipeline Flow -->
    <div v-if="!chainNodes.length" class="chain-empty-state">
      <p>尚未添加链路节点。请从下方代理节点列表中点击<strong>【+ 加入套接链】</strong>按连接顺序依次添加前置跳板与落地节点。</p>
    </div>
    <div v-else class="chain-pipeline-flow">
      <template v-for="(node, idx) in chainNodes" :key="node.id">
        <div class="chain-hop-card" :class="{ 'is-entry': idx === 0, 'is-exit': idx === chainNodes.length - 1 && chainNodes.length > 1 }">
          <div class="chain-hop-badge">
            <span class="hop-index">Hop {{ idx }}</span>
            <span v-if="idx === 0" class="hop-role-tag entry">前置中转 (跳板)</span>
            <span v-else-if="idx === chainNodes.length - 1" class="hop-role-tag exit">落地出口 (终端)</span>
            <span v-else class="hop-role-tag middle">中继跳板</span>
          </div>
          <div class="chain-hop-info">
            <strong class="chain-hop-name" :title="node.name">{{ node.name }}</strong>
            <span class="chain-hop-proto">{{ node.type === 'http' ? 'HTTP CONNECT' : node.type.toUpperCase() }}</span>
            <span class="chain-hop-addr" :title="`${node.server}:${node.port}`">{{ node.server }}:{{ node.port }}</span>
            <span v-if="idx === 0" class="chain-hop-egress">底层出口 {{ node.egress.toUpperCase() }}</span>
            <span v-else class="chain-hop-egress chained">经 Hop {{ idx - 1 }} 隧道</span>
          </div>
          <div class="chain-hop-actions">
            <button class="hop-btn" type="button" :disabled="isRunning || idx === 0" title="前移" @click="moveChainHop(idx, 'up', isRunning)">←</button>
            <button class="hop-btn" type="button" :disabled="isRunning || idx === chainNodes.length - 1" title="后移" @click="moveChainHop(idx, 'down', isRunning)">→</button>
            <button class="hop-btn remove" type="button" :disabled="isRunning" title="移出链路" @click="removeFromChain(idx, isRunning)">×</button>
          </div>
        </div>
        <div v-if="idx < chainNodes.length - 1" class="chain-arrow-connector" aria-hidden="true">
          <span class="chain-arrow-head">▶</span>
        </div>
      </template>
    </div>

    <!-- Chain Status Alert -->
    <div v-if="chainNodes.length === 1" class="chain-alert warning">
      <span>⚠️ 套接模式至少需要 2 个节点（1 个前置跳板 + 1 个落地节点）。请继续从下方添加落地节点。</span>
    </div>
    <div v-else-if="chainNodes.length >= 2 && chainNodes[0] && chainNodes[chainNodes.length - 1]" class="chain-alert info">
      <div class="chain-alert-left">
        <span>链路就绪：从 <strong>{{ routingMode === 'single' ? '主网卡' : `网卡 ${chainNodes[0]?.egress?.toUpperCase() ?? 'B'}` }}</strong> 发起连接 ➔ <strong>{{ chainNodes[0]?.name }}</strong> ➔ 落地于 <strong>{{ chainNodes[chainNodes.length - 1]?.name }}</strong></span>
      </div>
      <div v-if="chainTestResult" class="chain-test-badge" :class="chainTestResult.available ? 'test-ok' : 'test-failed'">
        <template v-if="chainTestResult.available">
          整链可用 · 延迟 {{ chainTestResult.latency_ms }} ms
          <small v-if="chainTestResult.tcp_ms"> (前置 TCP {{ chainTestResult.tcp_ms }}ms / 落地隧道 {{ chainTestResult.protocol_ms }}ms)</small>
        </template>
        <template v-else>
          整链不可用 · {{ formatErrorCategory(chainTestResult.error_category) }}{{ chainTestResult.error ? ` (${chainTestResult.error})` : '' }}
        </template>
      </div>
    </div>
  </section>
</template>
