<script setup lang="ts">
import { customRules } from '../../composables/useCustomRules'
import {
  processRuleStatuses,
  refreshRulePreview,
  rulePreview,
  rulePreviewError,
} from '../../composables/useRulePreview'
</script>

<template>
  <p v-if="customRules.some(rule => rule.type.startsWith('process-'))" class="process-note">
    进程名称可跨重启保持匹配；同名程序请使用完整路径。子进程不会隐式继承，请为实际联网的子进程单独添加规则。路径变化或无法识别时流量仍受基础域名/IP策略约束。
  </p>

  <section v-if="processRuleStatuses.length" class="process-statuses" aria-label="进程规则状态">
    <div
      v-for="status in processRuleStatuses"
      :key="`${status.type}-${status.value}`"
      :class="status.state"
    >
      <strong>{{ status.value }}</strong>
      <span>{{ status.message }}</span>
      <small v-if="status.paths?.length">{{ status.paths.join('、') }}</small>
    </div>
  </section>

  <section class="final-rules">
    <div class="rules-heading">
      <div>
        <p class="section-kicker">实际生成结果</p>
        <h2>最终规则顺序</h2>
        <p>包含基础设施、用户规则、直连前缀、内置规则及最终出口。</p>
      </div>
      <button class="secondary" type="button" @click="refreshRulePreview">刷新预览</button>
    </div>
    <p v-if="rulePreviewError" class="field-error">冲突诊断：{{ rulePreviewError }}</p>
    <div class="final-rule-list">
      <div v-for="item in rulePreview" :key="item.position">
        <span>{{ item.position }}</span>
        <strong>{{ item.category }}</strong>
        <code>{{ item.match.join('、') }}</code>
        <small>{{ item.action }}</small>
      </div>
    </div>
  </section>
</template>
