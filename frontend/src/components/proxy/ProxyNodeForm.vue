<script setup lang="ts">
import { isRunning } from '../../composables/useCoreManager'
import { busy } from '../../composables/useFeedback'
import { routingMode } from '../../composables/useNetworkInterfaces'
import {
  proxyFieldErrors,
  proxyForm,
  proxyFormOpen,
  proxyProtocols,
  resetProxyForm,
  saveProxyNode,
  selectProtocol,
  ssMethods,
} from '../../composables/useProxyNodes'
</script>

<template>
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
</template>
