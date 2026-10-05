<script setup lang="ts">
import ProxyChainPipeline from '../components/proxy/ProxyChainPipeline.vue'
import ProxyImportModal from '../components/proxy/ProxyImportModal.vue'
import ProxyNodeCard from '../components/proxy/ProxyNodeCard.vue'
import ProxyNodeForm from '../components/proxy/ProxyNodeForm.vue'
import ProxySubscriptionForm from '../components/proxy/ProxySubscriptionForm.vue'
import { isRunning } from '../composables/useCoreManager'
import { routingMode } from '../composables/useNetworkInterfaces'
import { isChainMode, switchProxyMode } from '../composables/useProxyChain'
import {
  closeImportBox,
  openImportBox,
  openNewProxyForm,
  proxyFormOpen,
  proxyImportOpen,
  proxyNodes,
  proxyRiskMessages,
  proxySort,
  resetProxyForm,
  sortedProxyNodes,
} from '../composables/useProxyNodes'
import { speedTestAllProxies, testingAllProxies } from '../composables/useProxyProbe'
import { openPinModal, securityStatus } from '../composables/useSecurityVault'
import {
  deleteSubscription,
  editSubscription,
  refreshSubscription,
  refreshingSubscriptionID,
  resetSubscriptionForm,
  subscriptionFormOpen,
  subscriptionList,
} from '../composables/useSubscriptions'
</script>

<template>
  <section v-if="proxyRiskMessages.length" class="proxy-risks" aria-label="代理风险">
    <div v-for="message in proxyRiskMessages" :key="message">
      <strong>需要处理</strong>
      <span>{{ message }}</span>
    </div>
    <small>代理 endpoint 同时使用出口 B 绑定和专用 /32 规则防止再次进入代理。</small>
  </section>

  <section
    v-if="securityStatus.pin_enabled && !securityStatus.unlocked"
    class="proxy-risks"
    style="border-left-color: #d69e2e;"
    aria-label="保险箱锁定提示"
  >
    <div>
      <strong>保险箱已锁定</strong>
      <span>节点凭据受 PIN 码安全保护，当前处于锁定状态。连接节点前请先输入 PIN 码解锁。</span>
    </div>
    <button type="button" class="secondary" style="margin-top: 8px;" @click="openPinModal('unlock')">
      输入 PIN 码解锁
    </button>
  </section>

  <section class="proxy-heading">
    <div>
      <p class="section-kicker">HTTP CONNECT / Shadowsocks / VMess / VLESS / Trojan</p>
      <h2>代理节点管理</h2>
      <p>节点凭据通过 AES-256-GCM 高强度加密本地保护，支持跨设备便携移动，界面和日志不会读取或显示原文。</p>
    </div>
    <div class="proxy-heading-actions">
      <label>
        排序
        <select v-model="proxySort">
          <option value="favorite">收藏优先</option>
          <option value="latency">延迟最低</option>
          <option value="name">名称</option>
        </select>
      </label>
      <button
        class="secondary"
        type="button"
        :disabled="testingAllProxies || !proxyNodes.length"
        @click="speedTestAllProxies"
      >
        {{ testingAllProxies ? '测速中' : '全部测速' }}
      </button>
      <button class="secondary" type="button" @click="proxyImportOpen ? closeImportBox() : openImportBox()">
        {{ proxyImportOpen ? '取消导入' : '从链接导入' }}
      </button>
      <button class="primary" type="button" @click="proxyFormOpen ? resetProxyForm() : openNewProxyForm()">
        {{ proxyFormOpen ? '取消编辑' : '添加节点' }}
      </button>
    </div>
  </section>

  <section class="proxy-mode-bar" aria-label="代理工作模式">
    <div class="proxy-mode-info">
      <span class="section-kicker">出口 C 代理工作模式</span>
      <strong>{{ isChainMode ? '链式套接模式 (Hop 0 ➔ Hop 1 ➔ 终端落地)' : '单节点直连模式' }}</strong>
      <p class="proxy-form-hint">
        {{
          isChainMode
            ? '多级跳板套接：前置节点通过指定物理出口（A 或 B）直连，后置节点通过前置代理隧道建立连接，最终落地于最后一跳。'
            : '单节点直连：直接通过指定物理网卡（A 或 B）连接选中的单一代理服务器。'
        }}
      </p>
    </div>
    <div class="mode-switch" role="tablist" aria-label="代理工作模式选择">
      <button
        type="button"
        :class="{ active: !isChainMode }"
        :disabled="isRunning"
        @click="switchProxyMode('single', isRunning)"
      >
        单节点直连
      </button>
      <button
        type="button"
        :class="{ active: isChainMode }"
        :disabled="isRunning || !proxyNodes.length"
        :title="!proxyNodes.length ? '请先添加或导入代理节点后再开启链式套接' : ''"
        @click="switchProxyMode('chain', isRunning)"
      >
        链式套接模式
      </button>
    </div>
  </section>

  <!-- Chain Pipeline Visualizer (Only visible in chain mode) -->
  <ProxyChainPipeline />

  <!-- Import from link card -->
  <ProxyImportModal />

  <!-- Node form -->
  <ProxyNodeForm />

  <section class="proxy-list" aria-label="代理节点列表">
    <div v-if="!proxyNodes.length" class="proxy-empty">尚未添加代理节点。</div>
    <ProxyNodeCard
      v-for="node in sortedProxyNodes"
      :key="node.id"
      :node="node"
    />
  </section>

  <section class="subscription-heading">
    <div>
      <p class="section-kicker">受限 JSON</p>
      <h2>节点订阅</h2>
      <p>公网地址强制 HTTPS；私网或回环 IP 可使用 HTTP。不执行脚本或命令，失败时保留上一批节点。</p>
    </div>
    <button class="secondary" type="button" @click="subscriptionFormOpen ? resetSubscriptionForm() : (subscriptionFormOpen = true)">
      {{ subscriptionFormOpen ? '取消' : '添加订阅' }}
    </button>
  </section>

  <!-- Subscription Form -->
  <ProxySubscriptionForm />

  <section class="subscription-list" aria-label="订阅列表">
    <div v-if="!subscriptionList.length" class="proxy-empty">尚未添加订阅。</div>
    <article v-for="item in subscriptionList" :key="item.id">
      <div>
        <strong>{{ item.name }}</strong>
        <small>{{ item.host }} · {{ item.node_count }} 个节点<template v-if="routingMode === 'dual'"> · 默认出口 {{ item.egress === 'a' ? 'A' : 'B' }}</template></small>
        <span v-if="item.last_error" class="test-failed">{{ item.last_error }}</span>
      </div>
      <div class="proxy-actions">
        <button
          class="secondary"
          type="button"
          :disabled="refreshingSubscriptionID === item.id"
          @click="refreshSubscription(item)"
        >
          {{ refreshingSubscriptionID === item.id ? '更新中' : '更新' }}
        </button>
        <button class="secondary" type="button" @click="editSubscription(item)">编辑</button>
        <button class="delete-button" type="button" @click="deleteSubscription(item)">删除</button>
      </div>
    </article>
  </section>
</template>
