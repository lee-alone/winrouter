<script setup lang="ts">
import { isRunning } from '../composables/useCoreManager'
import { busy } from '../composables/useFeedback'
import { routingMode } from '../composables/useNetworkInterfaces'
import {
  addToChain,
  applyImportURI,
  chainNodeHopIndex,
  chainNodes,
  chainTestResult,
  clearChain,
  closeImportBox,
  copyProxyNodeInfo,
  deleteProxyNode,
  deleteSubscription,
  editProxyNode,
  editSubscription,
  formatErrorCategory,
  formatProxySummary,
  isChainMode,
  isNodeInChain,
  moveChainHop,
  openImportBox,
  openNewProxyForm,
  proxyFieldErrors,
  proxyForm,
  proxyFormOpen,
  proxyImportError,
  proxyImportOpen,
  proxyImportURI,
  proxyNodes,
  proxyProtocols,
  proxyRiskMessages,
  proxySelection,
  proxySort,
  proxyTests,
  refreshSubscription,
  refreshingSubscriptionID,
  removeFromChain,
  resetProxyForm,
  resetSubscriptionForm,
  saveProxyNode,
  saveSubscription,
  selectProtocol,
  selectProxyNode,
  sortedProxyNodes,
  speedTestAllProxies,
  ssMethods,
  subscriptionForm,
  subscriptionFormOpen,
  subscriptionList,
  switchProxyMode,
  testChain,
  testProxy,
  testingAllProxies,
  testingChain,
  testingProxyID,
  toggleProxyFavorite,
  toggleProxyNodeEgress,
} from '../composables/useProxyManager'
import { openPinModal, securityStatus } from '../composables/useSecurityVault'
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

  <!-- Import from link card -->
  <section v-if="proxyImportOpen" class="proxy-import-box" aria-label="从链接导入节点">
    <div>
      <strong>从节点分享链接导入</strong>
      <p class="proxy-form-hint">支持 ss://、vmess://、vless://、trojan://、http:// 等单节点链接，解析后将自动填充表单供您核对与保存。</p>
    </div>
    <textarea v-model="proxyImportURI" placeholder="粘贴节点链接，例如：vless://... 或 ss://..."></textarea>
    <p v-if="proxyImportError" class="field-error-msg">{{ proxyImportError }}</p>
    <div class="proxy-import-actions">
      <button class="secondary" type="button" @click="closeImportBox">取消</button>
      <button class="primary" type="button" :disabled="!proxyImportURI.trim()" @click="applyImportURI">解析并填入表单</button>
    </div>
  </section>

  <!-- Node form -->
  <form v-if="proxyFormOpen" class="proxy-form" @submit.prevent="saveProxyNode(isRunning)">
    <!-- Protocol Selector Bar -->
    <div class="protocol-selector-row">
      <span class="protocol-selector-label">选择节点协议（点击直接切换对应协议的全部专属配置字段）</span>
      <div class="protocol-switch" role="tablist" aria-label="协议类型选择">
        <button
          v-for="proto in proxyProtocols"
          :key="proto.value"
          type="button"
          :class="{ active: proxyForm.type === proto.value }"
          @click="selectProtocol(proto.value)"
        >
          {{ proto.label }}
        </button>
      </div>
    </div>

    <label>
      节点名称
      <input v-model.trim="proxyForm.name" required maxlength="80" placeholder="例如：办公代理">
      <span v-if="proxyFieldErrors.name" class="field-error-msg">{{ proxyFieldErrors.name }}</span>
    </label>
    <label v-if="routingMode === 'dual'">
      物理出口
      <select v-model="proxyForm.egress">
        <option value="b">接口 B（默认）</option>
        <option value="a">接口 A</option>
      </select>
    </label>
    <label>
      服务器地址
      <input v-model.trim="proxyForm.server" required placeholder="203.0.113.10 或 proxy.example.com">
      <span v-if="proxyFieldErrors.server" class="field-error-msg">{{ proxyFieldErrors.server }}</span>
    </label>
    <label>
      端口
      <input v-model.number="proxyForm.port" required type="number" min="1" max="65535">
      <span v-if="proxyFieldErrors.port" class="field-error-msg">{{ proxyFieldErrors.port }}</span>
    </label>

    <!-- Credential Management for Existing Nodes -->
    <div v-if="proxyForm.id" class="credential-status-box">
      <div class="credential-status-header">
        <div>
          <strong>{{ proxyForm.has_saved_secret ? '🔒 节点凭据状态：已加密保存' : '节点凭据状态：未配置凭据' }}</strong>
          <small v-if="proxyForm.has_saved_secret">凭据通过高强度加密本地保护，支持便携移动，无需重复输入。</small>
        </div>
        <div class="credential-controls">
          <label v-if="proxyForm.has_saved_secret" class="toggle">
            <input v-model="proxyForm.replace_secret" type="checkbox" :disabled="proxyForm.clear_secret">
            <span>{{ proxyForm.replace_secret ? '替换凭据' : '保留原凭据' }}</span>
          </label>
          <label v-if="proxyForm.type === 'http' && proxyForm.has_saved_secret" class="clear-secret">
            <input
              v-model="proxyForm.clear_secret"
              type="checkbox"
              @change="proxyForm.clear_secret && (proxyForm.replace_secret = false)"
            >
            <span>清除已保存凭据</span>
          </label>
        </div>
      </div>
      <p v-if="proxyForm.id" class="proxy-operation-indicator">
        {{
          proxyForm.type !== proxyForm.original_type
            ? proxyForm.password || proxyForm.uuid
              ? '操作预期：协议已切换，保存后将使用新凭据覆盖原有凭据。'
              : '操作预期：协议已切换，未提供新密码/凭据，保存后将清除原协议的加密凭据。'
            : proxyForm.clear_secret
            ? '操作预期：保存后将清除已保存的密码与认证凭据。'
            : proxyForm.replace_secret
            ? '操作预期：保存后将使用下方新输入的凭据覆盖原有凭据。'
            : '操作预期：保存后将保留原有加密凭据不变。'
        }}
      </p>
    </div>

    <!-- HTTP Fields -->
    <template v-if="proxyForm.type === 'http'">
      <label>
        用户名
        <input v-model.trim="proxyForm.username" autocomplete="off" placeholder="可选用户名">
      </label>
      <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
        密码
        <input
          v-model="proxyForm.password"
          type="password"
          autocomplete="new-password"
          :disabled="proxyForm.clear_secret"
          :placeholder="proxyForm.id ? '输入新密码' : '可选密码'"
        >
        <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
      </label>
    </template>

    <!-- Shadowsocks Fields -->
    <template v-else-if="proxyForm.type === 'shadowsocks'">
      <label>
        加密方法
        <select v-model="proxyForm.method" required>
          <option v-for="method in ssMethods" :key="method" :value="method">{{ method }}</option>
        </select>
      </label>
      <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
        密码
        <input
          v-model="proxyForm.password"
          type="password"
          autocomplete="new-password"
          :required="!proxyForm.id || proxyForm.replace_secret"
          placeholder="必填密码"
        >
        <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
      </label>
    </template>

    <!-- VMess Fields -->
    <template v-else-if="proxyForm.type === 'vmess'">
      <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
        UUID
        <input
          v-model.trim="proxyForm.uuid"
          autocomplete="off"
          :required="!proxyForm.id || proxyForm.replace_secret"
          placeholder="例如：a8e678c0-51c0-4212-9c17-1f4bf7ec0000"
        >
        <span v-if="proxyFieldErrors.uuid" class="field-error-msg">{{ proxyFieldErrors.uuid }}</span>
      </label>
    </template>

    <!-- VLESS Fields -->
    <template v-else-if="proxyForm.type === 'vless'">
      <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
        UUID
        <input
          v-model.trim="proxyForm.uuid"
          autocomplete="off"
          :required="!proxyForm.id || proxyForm.replace_secret"
          placeholder="例如：a8e678c0-51c0-4212-9c17-1f4bf7ec0000"
        >
        <span v-if="proxyFieldErrors.uuid" class="field-error-msg">{{ proxyFieldErrors.uuid }}</span>
      </label>
      <label>
        Flow
        <select v-model="proxyForm.flow">
          <option value="">无 (none)</option>
          <option value="xtls-rprx-vision">xtls-rprx-vision</option>
        </select>
      </label>
    </template>

    <!-- Trojan Fields -->
    <template v-else-if="proxyForm.type === 'trojan'">
      <label v-if="!proxyForm.id || proxyForm.replace_secret || !proxyForm.has_saved_secret">
        密码
        <input
          v-model="proxyForm.password"
          type="password"
          autocomplete="new-password"
          :required="!proxyForm.id || proxyForm.replace_secret"
          placeholder="必填密码"
        >
        <span v-if="proxyFieldErrors.password" class="field-error-msg">{{ proxyFieldErrors.password }}</span>
      </label>
    </template>

    <!-- TLS settings for VMess / VLESS / Trojan -->
    <template v-if="proxyForm.type === 'vmess' || proxyForm.type === 'vless' || proxyForm.type === 'trojan'">
      <label class="clear-secret">
        <input v-if="proxyForm.type !== 'trojan'" v-model="proxyForm.tls_enabled" type="checkbox">
        <input v-else type="checkbox" checked disabled>
        <span>{{ proxyForm.type === 'trojan' ? '强制启用 TLS（Trojan 协议规范）' : '启用 TLS' }}</span>
      </label>
      <template v-if="proxyForm.type === 'trojan' || proxyForm.tls_enabled">
        <label>
          TLS Server Name (SNI)
          <input v-model.trim="proxyForm.tls_server_name" placeholder="留空默认使用服务器地址">
          <span v-if="proxyFieldErrors.tls_server_name" class="field-error-msg">{{ proxyFieldErrors.tls_server_name }}</span>
        </label>
        <label>
          TLS ALPN
          <input v-model.trim="proxyForm.tls_alpn" placeholder="例如：h2, http/1.1（逗号分隔）">
          <span v-if="proxyFieldErrors.tls_alpn" class="field-error-msg">{{ proxyFieldErrors.tls_alpn }}</span>
        </label>
        <label class="clear-secret">
          <input v-model="proxyForm.tls_insecure" type="checkbox">
          <span>允许不安全证书 (Insecure)</span>
        </label>
      </template>

      <!-- Transport Settings -->
      <label>
        传输协议
        <select v-model="proxyForm.transport_type">
          <option value="tcp">TCP</option>
          <option value="ws">WebSocket (WS)</option>
        </select>
      </label>
      <template v-if="proxyForm.transport_type === 'ws'">
        <label>
          WebSocket Path
          <input v-model.trim="proxyForm.transport_path" placeholder="必须以 / 开头，例如：/ws 或 /chat">
          <span v-if="proxyFieldErrors.transport_path" class="field-error-msg">{{ proxyFieldErrors.transport_path }}</span>
        </label>
        <label>
          WebSocket Host
          <input v-model.trim="proxyForm.transport_host" placeholder="例如：proxy.example.com">
          <span v-if="proxyFieldErrors.transport_host" class="field-error-msg">{{ proxyFieldErrors.transport_host }}</span>
        </label>
      </template>
    </template>

    <div class="proxy-form-actions">
      <button class="secondary" type="button" @click="resetProxyForm">取消</button>
      <button class="primary" type="submit" :disabled="busy">
        {{ busy ? '正在保存' : proxyForm.id ? '保存修改' : '添加节点' }}
      </button>
    </div>
  </form>

  <section class="proxy-list" aria-label="代理节点列表">
    <div v-if="!proxyNodes.length" class="proxy-empty">尚未添加代理节点。</div>
    <article
      v-for="node in sortedProxyNodes"
      :key="node.id"
      :class="{ selected: isChainMode ? isNodeInChain(node.id) : node.selected }"
    >
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

  <form v-if="subscriptionFormOpen" class="subscription-form" @submit.prevent="saveSubscription">
    <label>
      订阅名称
      <input v-model.trim="subscriptionForm.name" required maxlength="80">
    </label>
    <label>
      HTTPS / 局域网 HTTP 地址
      <input
        v-model.trim="subscriptionForm.url"
        :required="!subscriptionForm.id"
        type="url"
        :placeholder="subscriptionForm.id ? '留空则保留加密地址' : 'https://example.com/nodes.json 或 http://192.168.1.50/nodes.json'"
      >
    </label>
    <label v-if="routingMode === 'dual'">
      默认出口网卡
      <select v-model="subscriptionForm.egress">
        <option value="b">出口 B (海外/代理出口)</option>
        <option value="a">出口 A (国内/直连出口)</option>
      </select>
    </label>
    <div>
      <button class="secondary" type="button" @click="resetSubscriptionForm">取消</button>
      <button class="primary" type="submit" :disabled="busy">保存</button>
    </div>
  </form>

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
