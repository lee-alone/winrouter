<script setup lang="ts">
import type { nodes } from '../../../wailsjs/go/models'
import { isRunning } from '../../composables/useCoreManager'
import { routingMode } from '../../composables/useNetworkInterfaces'
import {
  addToChain,
  chainNodeHopIndex,
  isChainMode,
  isNodeInChain,
  proxySelection,
  removeFromChain,
} from '../../composables/useProxyChain'
import {
  copyProxyNodeInfo,
  deleteProxyNode,
  editProxyNode,
  formatProxySummary,
  selectProxyNode,
  toggleProxyFavorite,
  toggleProxyNodeEgress,
} from '../../composables/useProxyNodes'
import {
  formatErrorCategory,
  proxyTests,
  testingProxyID,
  testProxy,
} from '../../composables/useProxyProbe'

defineProps<{
  node: nodes.Node
}>()
</script>

<template>
  <article :class="{ selected: isChainMode ? isNodeInChain(node.id) : node.selected }">
    <div class="proxy-status">
      <button
        class="favorite-button"
        type="button"
        :class="{ active: node.favorite }"
        :aria-label="node.favorite ? '取消收藏' : '收藏节点'"
        :title="node.favorite ? '取消收藏' : '收藏节点'"
        @click="toggleProxyFavorite(node)"
      >
        {{ node.favorite ? '★' : '☆' }}
      </button>
      <span
        :class="['selection-dot', { active: isChainMode ? isNodeInChain(node.id) : node.selected }]"
        aria-hidden="true"
      ></span>
      <div>
        <strong>{{ node.name }}</strong>
        <small>{{ node.type === 'http' ? 'HTTP CONNECT' : node.type.toUpperCase() }} · {{ node.server }}:{{ node.port }}<template v-if="node.resolved_ip && node.resolved_ip !== node.server"> → {{ node.resolved_ip }}</template></small>
      </div>
    </div>
    <div class="proxy-meta">
      <span v-if="isChainMode && isNodeInChain(node.id)" class="chain-pos-badge">
        第 {{ chainNodeHopIndex(node.id) + 1 }} 跳 {{ chainNodeHopIndex(node.id) === 0 ? '(前置跳板)' : (chainNodeHopIndex(node.id) === (proxySelection?.selected_chain?.length || 0) - 1 && (proxySelection?.selected_chain?.length || 0) > 1) ? '(落地出口)' : '' }}
      </span>
      <span
        v-if="routingMode === 'dual'"
        :class="['proxy-meta-tag', { 'overridden-egress-tag': node.egress_overridden }]"
        :title="node.egress_overridden ? '此节点已单独自定义出口网卡' : undefined"
      >
        {{ node.egress === 'a' ? '出口 A' : '出口 B' }}<template v-if="node.egress_overridden"> (已覆盖)</template>
      </span>
      <span v-else class="proxy-meta-tag">主网卡出站</span>
      <span v-for="tag in formatProxySummary(node)" :key="tag" class="proxy-meta-tag">{{ tag }}</span>
      <span :class="['credential-tag', { none: !(node.has_secret || node.has_password) }]">
        {{ node.has_secret || node.has_password ? '🔒 已保存凭据' : '未设凭据' }}
      </span>
      <span v-if="proxyTests[node.id]" :class="proxyTests[node.id].available ? 'test-ok' : 'test-failed'">
        <template v-if="proxyTests[node.id].available">
          可用 · {{ proxyTests[node.id].total_ms || proxyTests[node.id].latency_ms }} ms
          <small v-if="proxyTests[node.id].tcp_ms"> (TCP {{ proxyTests[node.id].tcp_ms }}ms / 协议 {{ proxyTests[node.id].protocol_ms }}ms)</small>
        </template>
        <template v-else>
          不可用 · {{ formatErrorCategory(proxyTests[node.id].error_category) }}
        </template>
      </span>
      <span v-else>尚未测试</span>
    </div>
    <div class="proxy-actions">
      <button
        class="secondary"
        type="button"
        :disabled="testingProxyID === node.id"
        @click="testProxy(node)"
      >
        {{ testingProxyID === node.id ? '测试中' : '测试' }}
      </button>
      <template v-if="isChainMode">
        <button
          v-if="!isNodeInChain(node.id)"
          class="primary-subtle"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止修改链路' : '将此节点添加到套接链路末端'"
          @click="addToChain(node, isRunning)"
        >
          + 加入套接链
        </button>
        <button
          v-else
          class="secondary"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止修改链路' : '从套接链路中移除'"
          @click="removeFromChain(chainNodeHopIndex(node.id), isRunning)"
        >
          移出链路
        </button>
      </template>
      <template v-else>
        <button
          v-if="!node.selected"
          class="secondary"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止切换节点' : '选择此节点'"
          @click="selectProxyNode(node.id, isRunning)"
        >
          选择
        </button>
        <span v-else class="selected-label">当前节点</span>
      </template>
      <button class="secondary" type="button" title="复制节点配置摘要" @click="copyProxyNodeInfo(node)">复制</button>
      <template v-if="!node.subscription_id">
        <button
          class="secondary"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止修改节点' : '编辑节点'"
          @click="editProxyNode(node)"
        >
          编辑
        </button>
        <button
          class="delete-button"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止删除节点' : '删除节点'"
          @click="deleteProxyNode(node, isRunning)"
        >
          删除
        </button>
      </template>
      <template v-else>
        <button
          v-if="routingMode === 'dual'"
          class="secondary"
          type="button"
          :disabled="isRunning"
          :title="isRunning ? '核心运行中，禁止修改出口' : `切换此节点至出口 ${node.egress === 'a' ? 'B' : 'A'}`"
          @click="toggleProxyNodeEgress(node, isRunning)"
        >
          切到出口 {{ node.egress === 'a' ? 'B' : 'A' }}
        </button>
        <span class="subscription-badge">订阅节点</span>
      </template>
    </div>
  </article>
</template>
