<script setup lang="ts">
import { busy } from '../../composables/useFeedback'
import { routingMode } from '../../composables/useNetworkInterfaces'
import {
  resetSubscriptionForm,
  saveSubscription,
  subscriptionForm,
  subscriptionFormOpen,
} from '../../composables/useSubscriptions'
</script>

<template>
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
</template>
