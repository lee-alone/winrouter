<script setup lang="ts">
import {
  deleteCustomRule,
  editCustomRule,
  formatRuleAction,
  masterRows,
  moveMasterRule,
  ruleTypeText,
  toggleCustomRule,
} from '../../composables/useCustomRules'
import {
  deleteSRSSource,
  editSRSSource,
  formatSRSUpdatedAt,
  refreshSRSSource,
  srsBusyID,
  toggleSRSSource,
} from '../../composables/useSRSRules'
import type { RuleAction } from '../../types'
</script>

<template>
  <section class="rule-master-list" aria-label="规则总表">
    <div class="rules-heading">
      <div>
        <p class="section-kicker">唯一编辑入口</p>
        <h2>分流规则总表</h2>
        <p>规则按从上到下的顺序匹配，第一条命中后停止；网卡 A/B 区域仅展示生成结果。</p>
      </div>
    </div>
    <div v-if="!masterRows.length" class="outlet-rule-empty">尚未添加规则</div>
    <article
      v-for="(row, index) in masterRows"
      :key="row.key"
      :class="['master-rule-item', { disabled: row.kind === 'custom' ? !row.rule.enabled : !row.source.enabled }]"
    >
      <strong class="master-rule-order">{{ index + 1 }}</strong>
      <template v-if="row.kind === 'custom'">
        <label class="master-rule-enabled">
          <input type="checkbox" :checked="row.rule.enabled" @change="toggleCustomRule(row.rule)">
          <span>{{ row.rule.enabled ? '启用' : '停用' }}</span>
        </label>
        <div>
          <strong>{{ row.rule.name }}</strong>
          <small>{{ ruleTypeText(row.rule.type) }} · {{ row.rule.values.length }} 个值 · {{ row.rule.values.slice(0, 3).join('、') }}{{ row.rule.values.length > 3 ? '…' : '' }}</small>
        </div>
        <span>{{ formatRuleAction(row.rule.action) }}</span>
        <div class="proxy-actions">
          <button class="secondary" type="button" :disabled="index === 0" @click="moveMasterRule(row.key, -1)">上移</button>
          <button class="secondary" type="button" :disabled="index === masterRows.length - 1" @click="moveMasterRule(row.key, 1)">下移</button>
          <button class="secondary" type="button" @click="editCustomRule(row.rule)">编辑</button>
          <button v-if="row.rule.id !== 'default-private-lan'" class="delete-button" type="button" @click="deleteCustomRule(row.rule.id)">删除</button>
        </div>
      </template>
      <template v-else>
        <label class="master-rule-enabled">
          <input type="checkbox" :checked="row.source.enabled" :disabled="srsBusyID === row.source.id" @change="toggleSRSSource(row.source)">
          <span>{{ row.source.enabled ? '启用' : '停用' }}</span>
        </label>
        <div>
          <strong>{{ row.source.name }}</strong>
          <small>{{ row.source.kind === 'domain' ? 'geosite / 域名集合' : 'geoip / IP 集合' }} · {{ row.source.applied_sha256 ? '数据集已验证' : row.source.last_error ? '下载失败，暂无缓存' : '待下载' }}</small>
          <small>{{ formatSRSUpdatedAt(row.source) }}<template v-if="row.source.last_error && row.source.applied_sha256"> · 上次更新失败，继续使用有效缓存</template></small>
        </div>
        <span>{{ formatRuleAction(row.source.action as RuleAction) }}</span>
        <div class="proxy-actions">
          <button class="secondary" type="button" :disabled="index === 0" @click="moveMasterRule(row.key, -1)">上移</button>
          <button class="secondary" type="button" :disabled="index === masterRows.length - 1" @click="moveMasterRule(row.key, 1)">下移</button>
          <button class="secondary" type="button" @click="editSRSSource(row.source)">编辑</button>
          <button class="secondary" type="button" :disabled="srsBusyID === row.source.id" @click="refreshSRSSource(row.source)">{{ srsBusyID === row.source.id ? '校验中' : '更新' }}</button>
          <button v-if="!row.source.preset_id" class="delete-button" type="button" @click="deleteSRSSource(row.source)">删除</button>
        </div>
      </template>
    </article>
  </section>
</template>
