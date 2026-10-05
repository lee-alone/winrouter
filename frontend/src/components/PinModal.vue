<script setup lang="ts">
import {
  closePinModal,
  oldPinInput,
  pinInput,
  pinModalBusy,
  pinModalError,
  pinModalMode,
  pinModalOpen,
  submitPinModal,
} from '../composables/useSecurityVault'
</script>

<template>
  <div v-if="pinModalOpen" class="pin-modal-backdrop" @click.self="closePinModal">
    <div class="pin-modal-card">
      <div class="pin-modal-header">
        <h3>
          <span v-if="pinModalMode === 'unlock'">🔑 解锁节点凭据保险箱</span>
          <span v-else-if="pinModalMode === 'enable'">🛡️ 启用 PIN 码安全保护</span>
          <span v-else-if="pinModalMode === 'change'">🔄 修改 PIN 码</span>
          <span v-else-if="pinModalMode === 'disable'">⚠️ 停用 PIN 码保护</span>
        </h3>
        <button type="button" class="icon-button" @click="closePinModal">✕</button>
      </div>
      <div class="pin-modal-body">
        <p class="pin-modal-desc">
          <span v-if="pinModalMode === 'unlock'">请输入安全 PIN 码以解密代理节点和订阅信息。</span>
          <span v-else-if="pinModalMode === 'enable'">设置变长 PIN 码（至少 4 位字符）。设置后，节点密码将由 PIN 码派生密钥高强度加密密封。</span>
          <span v-else-if="pinModalMode === 'change'">请输入当前 PIN 码以验证身份，并输入新的安全 PIN 码。</span>
          <span v-else-if="pinModalMode === 'disable'">输入当前 PIN 码验证后，将恢复为基于本地密钥的便携免密模式。</span>
        </p>
        <div v-if="pinModalMode === 'change'" class="pin-input-group">
          <label>原 PIN 码</label>
          <input v-model="oldPinInput" type="password" placeholder="输入原 PIN 码" autocomplete="current-password">
        </div>
        <div class="pin-input-group">
          <label>{{ pinModalMode === 'change' ? '新 PIN 码' : (pinModalMode === 'disable' ? '验证当前 PIN 码' : '安全 PIN 码') }}</label>
          <input v-model="pinInput" type="password" placeholder="请输入 PIN 码" autocomplete="new-password" @keyup.enter="submitPinModal">
        </div>
        <p v-if="pinModalError" class="field-error-msg" style="margin-top: 8px;">{{ pinModalError }}</p>
      </div>
      <div class="pin-modal-footer">
        <button type="button" class="secondary" :disabled="pinModalBusy" @click="closePinModal">取消</button>
        <button type="button" class="primary" :disabled="pinModalBusy || !pinInput.trim()" @click="submitPinModal">
          {{ pinModalBusy ? '处理中...' : '确认' }}
        </button>
      </div>
    </div>
  </div>
</template>
