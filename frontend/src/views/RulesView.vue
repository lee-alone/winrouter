<script setup lang="ts">
import RuleDNSSection from '../components/rules/RuleDNSSection.vue'
import RuleForm from '../components/rules/RuleForm.vue'
import RuleMasterTable from '../components/rules/RuleMasterTable.vue'
import RulePolicyHeader from '../components/rules/RulePolicyHeader.vue'
import RulePreviewSection from '../components/rules/RulePreviewSection.vue'
import {
  resetRuleForm,
  ruleFormOpen,
} from '../composables/useCustomRules'
</script>

<template>
  <!-- 顶部网卡出口映射、DNS策略摘要及兜底出口选择器 -->
  <RulePolicyHeader />

  <!-- 规则模块标题与新建控制 -->
  <section class="rules-heading">
    <div>
      <p class="section-kicker">按实际出口配置</p>
      <h2>用户覆盖规则</h2>
      <p>为 IP、域名、进程或 SRS 规则集指定出口；系统防环路和本地网络规则始终优先。</p>
    </div>
    <button
      class="primary"
      type="button"
      @click="ruleFormOpen ? resetRuleForm() : (ruleFormOpen = true)"
    >
      {{ ruleFormOpen ? '取消' : '添加规则' }}
    </button>
  </section>

  <!-- 规则新建与编辑表单（自定义规则 / SRS 规则集） -->
  <RuleForm />

  <!-- 分流规则总表与排序列表 -->
  <RuleMasterTable />

  <!-- 各出口独立 DNS 预设与测速配置 -->
  <RuleDNSSection />

  <!-- 底层路由编译预览及进程规则状态 -->
  <RulePreviewSection />
</template>
