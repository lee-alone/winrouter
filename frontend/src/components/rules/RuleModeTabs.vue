<script setup lang="ts">
import {
  selectedRuleMode,
  switchRuleMode,
} from '../../composables/useRuleProfiles'
import { routingMode } from '../../composables/useNetworkInterfaces'
</script>

<template>
  <section class="mode-row" aria-label="规则模式选择">
    <div>
      <strong>分流规则配置库</strong>
      <p style="margin: 4px 0 0; color: #546562; font-size: 13px;">
        单网卡与双网卡使用独立的规则配置；启动核心时自动按网卡界面的配置模式加载生效。
      </p>
    </div>
    <div style="display: flex; align-items: center; gap: 12px; flex-wrap: wrap;">
      <span
        v-if="selectedRuleMode === routingMode"
        style="padding: 4px 10px; font-size: 12px; background: #e0f2fe; color: #0369a1; border-radius: 4px; font-weight: 600;"
      >
        当前网卡模式生效中
      </span>
      <span
        v-else
        style="padding: 4px 10px; font-size: 12px; background: #fef3c7; color: #92400e; border-radius: 4px; font-weight: 600;"
      >
        查看未生效模式 (网卡界面当前为{{ routingMode === 'single' ? '单网卡' : '双网卡' }})
      </span>
      <div class="mode-switch">
        <button
          type="button"
          :class="{ active: selectedRuleMode === 'single' }"
          @click="switchRuleMode('single')"
        >
          单网卡规则配置
        </button>
        <button
          type="button"
          :class="{ active: selectedRuleMode === 'dual' }"
          @click="switchRuleMode('dual')"
        >
          双网卡规则配置
        </button>
      </div>
    </div>
  </section>
</template>
