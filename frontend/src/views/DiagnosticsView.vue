<script setup lang="ts">
import { onMounted } from 'vue'
import { filteredLogs, levelText, logFilter } from '../composables/useAppLogs'
import {
  applyRecommendation,
  diagnosticPreview,
  diagnosticRecommendations,
  exportDiagnostics,
  previewDiagnostics,
  probeDNSName,
  probeProtocol,
  probeRole,
  probeTarget,
  recommendationActionText,
  runProbe,
} from '../composables/useDiagnostics'
import { busy } from '../composables/useFeedback'
import { routingMode } from '../composables/useNetworkInterfaces'
import { observations, refreshObservations } from '../composables/useObservability'

onMounted(async () => {
  await refreshObservations()
})
</script>

<template>
  <section class="diagnostic-assessment" aria-label="diagnostic assessment">
    <div class="assessment-heading">
      <div>
        <p class="section-kicker">诊断结果</p>
        <h2>问题与修复建议</h2>
      </div>
      <small>建议只提供安全的下一步，不会自动修改网卡或绕过授权。</small>
    </div>
    <article
      v-for="item in diagnosticRecommendations"
      :key="`${item.title}-${item.detail}`"
      :class="['recommendation', item.severity]"
    >
      <div>
        <strong>{{ item.title }}</strong>
        <span>{{ item.detail }}</span>
      </div>
      <button
        v-if="item.action !== 'none'"
        class="secondary"
        type="button"
        @click="applyRecommendation(item.action)"
      >
        {{ recommendationActionText(item.action) }}
      </button>
    </article>
  </section>

  <section class="probe-panel">
    <div>
      <p class="section-kicker">主动健康探测</p>
      <h2>验证出口路径</h2>
    </div>
    <label>
      协议
      <select v-model="probeProtocol">
        <option value="tcp">TCP</option>
        <option value="udp">UDP</option>
        <option value="dns">DNS</option>
      </select>
    </label>
    <label>
      预期出口
      <select v-model="probeRole">
        <option value="A">{{ routingMode === 'single' ? '主网卡' : '出口 A' }}</option>
        <option v-if="routingMode === 'dual'" value="B">出口 B</option>
      </select>
    </label>
    <label class="probe-target">
      目标
      <input
        v-model="probeTarget"
        :placeholder="probeProtocol === 'dns' ? '223.5.5.5:53' : '1.1.1.1:443'"
      >
    </label>
    <label v-if="probeProtocol === 'dns'" class="probe-target">
      查询名称
      <input v-model="probeDNSName">
    </label>
    <button
      class="secondary"
      type="button"
      :disabled="busy || !probeTarget"
      @click="runProbe"
    >
      运行探测
    </button>
  </section>

  <section class="observation-strip" aria-label="诊断摘要">
    <div>
      <span>探测记录</span>
      <strong>{{ observations.probes.length }}</strong>
      <small>{{ observations.probes.at(-1)?.success ? '最近一次通过' : observations.probes.length ? '最近一次失败' : '尚未执行' }}</small>
    </div>
    <div>
      <span>接口计数器</span>
      <strong>{{ observations.counters.length }}</strong>
      <small>仅用于趋势辅助诊断</small>
    </div>
    <div>
      <span>Rule-set</span>
      <strong>{{ observations.rule_sets.length }}</strong>
      <small>{{ observations.rule_sets[0]?.load_result ?? '核心应用后记录' }}</small>
    </div>
  </section>

  <section class="log-toolbar">
    <div>
      <p class="section-kicker">运行事件</p>
      <h2>最近 200 条</h2>
    </div>
    <label>
      日志级别
      <select v-model="logFilter">
        <option value="all">全部</option>
        <option value="info">信息</option>
        <option value="warning">警告</option>
        <option value="error">错误</option>
      </select>
    </label>
  </section>

  <section class="log-list" aria-live="polite" aria-label="实时日志">
    <div v-if="!filteredLogs.length" class="log-empty">当前筛选条件下没有日志。</div>
    <article v-for="entry in filteredLogs" :key="entry.id" class="log-entry">
      <time>{{ entry.time }}</time>
      <span :class="['log-level', entry.level]">{{ levelText(entry.level) }}</span>
      <p>{{ entry.message }}</p>
      <code v-if="entry.correlation">{{ entry.correlation }}</code>
    </article>
  </section>

  <section class="export-band">
    <div>
      <p class="section-kicker">诊断包</p>
      <h2>脱敏预览与导出</h2>
      <p>完整 rule-set 二进制不会包含在导出内容中。</p>
    </div>
    <div class="export-actions">
      <button class="secondary" type="button" @click="previewDiagnostics">预览</button>
      <button
        class="primary"
        type="button"
        :disabled="busy || !diagnosticPreview"
        @click="exportDiagnostics"
      >
        导出 ZIP
      </button>
    </div>
  </section>

  <section v-if="diagnosticPreview" class="bundle-preview">
    <strong>{{ diagnosticPreview.files.length }} 个文件 · {{ diagnosticPreview.log_count }} 条日志 · {{ diagnosticPreview.probe_count }} 条探测</strong>
    <span>{{ diagnosticPreview.files.join('、') }}</span>
    <small v-for="note in diagnosticPreview.sensitive_notes" :key="note">{{ note }}</small>
  </section>
</template>
