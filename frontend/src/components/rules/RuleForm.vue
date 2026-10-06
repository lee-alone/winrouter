<script setup lang="ts">
import {
  resetRuleForm,
  ruleForm,
  ruleFormOpen,
  submitRule,
} from '../../composables/useCustomRules'
import {
  selectedAdapterA,
  selectedAdapterB,
} from '../../composables/useNetworkInterfaces'
import { selectedRuleMode } from '../../composables/useRuleProfiles'
import {
  chooseSRSPreset,
  srsBusyID,
  srsForm,
  srsPresets,
} from '../../composables/useSRSRules'
</script>

<template>
  <form v-if="ruleFormOpen" class="rule-form" @submit.prevent="submitRule">
    <div class="rule-form-primary">
      <label>
        名称
        <input v-model.trim="ruleForm.name" required maxlength="80" placeholder="例如：阻止广告域名">
      </label>
      <label>
        匹配类型
        <select v-model="ruleForm.type" :disabled="Boolean(ruleForm.id)">
          <option value="domain-suffix">域名后缀</option>
          <option value="domain">精确域名</option>
          <option value="ip">IPv4 CIDR</option>
          <option value="process-name">进程名称</option>
          <option value="process-path">进程完整路径</option>
          <option value="rule-set">SRS 规则集</option>
        </select>
      </label>
      <label v-if="ruleForm.type === 'rule-set'">
        来源预设
        <select v-model="srsForm.preset_id" @change="chooseSRSPreset">
          <option value="">自定义 HTTPS 地址</option>
          <option v-for="preset in srsPresets" :key="preset.id" :value="preset.id">{{ preset.name }}</option>
        </select>
      </label>
      <label>
        目标出口
        <select v-model="ruleForm.action">
          <option value="a">
            {{ selectedRuleMode === 'single' ? '直连 (主网卡) · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}
          </option>
          <option v-if="selectedRuleMode === 'dual'" value="b">
            出口 B · {{ selectedAdapterB?.friendly_name ?? '网卡 B' }}
          </option>
          <option value="c">出口 C · 代理出站</option>
          <option value="final">跟随默认出口</option>
          <option value="reject">阻断拒绝</option>
        </select>
      </label>
      <label class="toggle-label">
        <input v-model="ruleForm.enabled" type="checkbox">
        <span>启用规则</span>
      </label>
    </div>

    <label v-if="ruleForm.type !== 'rule-set'" class="rule-form-values">
      匹配值（每行一个）
      <textarea
        v-model="ruleForm.valuesText"
        required
        rows="6"
        placeholder="每行填写一个域名、CIDR 或进程匹配值"
      ></textarea>
    </label>

    <template v-if="ruleForm.type === 'rule-set'">
      <label>
        数据类型
        <select v-model="srsForm.kind" :disabled="Boolean(srsForm.preset_id)">
          <option value="domain">域名集合 / geosite</option>
          <option value="ip">IP 集合 / geoip</option>
        </select>
      </label>
      <label class="remote-url">
        固定 HTTPS 地址
        <input
          v-model.trim="srsForm.url"
          required
          type="url"
          :readonly="Boolean(srsForm.preset_id)"
          placeholder="https://example.com/rules.srs"
        >
      </label>
      <label class="remote-hash">
        固定 SHA-256（可选）
        <small>{{ srsForm.preset_id ? '内置滚动源由应用记录实际 hash' : '留空则允许滚动更新；填写后锁定指定版本' }}</small>
        <input
          v-model.trim="srsForm.expected_sha256"
          minlength="64"
          maxlength="64"
          placeholder="可留空"
        >
      </label>
    </template>

    <div class="rule-form-actions">
      <button class="secondary" type="button" @click="resetRuleForm">取消</button>
      <button class="primary" type="submit" :disabled="Boolean(srsBusyID)">
        {{ ruleForm.id ? '保存修改' : '添加规则' }}
      </button>
    </div>
  </form>
</template>
