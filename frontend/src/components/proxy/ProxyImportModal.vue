<script setup lang="ts">
import {
  applyImportURI,
  closeImportBox,
  proxyImportError,
  proxyImportOpen,
  proxyImportURI,
} from '../../composables/useProxyNodes'
</script>

<template>
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
</template>
