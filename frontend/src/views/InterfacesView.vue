<script setup lang="ts">
import { isRunning } from '../composables/useCoreManager'
import { busy } from '../composables/useFeedback'
import { setView } from '../composables/useNavigation'
import {
  candidates,
  directPrefixes,
  routingMode,
  sameAdapterWarning,
  saveSelection,
  selectedA,
  selectedAdapterA,
  selectedAdapterB,
  selectedB,
  setupRequired,
  snapshot,
  switchRoutingMode,
} from '../composables/useNetworkInterfaces'
import { t } from '../i18n'

function statusText(status: string) {
  return status === 'up' ? t('status.up') : status === 'down' ? t('status.down') : status
}

function formatList(values?: string[]) {
  return values?.length ? values.join(', ') : t('common.none')
}
</script>

<template>
  <section class="intro">
    <p>
      {{
        setupRequired
          ? '选择工作模式及可用物理网卡。系统会持久化接口 GUID，并在网络变化时重新核对。'
          : '查看或修改网络出口模式与绑定的网卡。修改已保存的出口前会要求确认。'
      }}
    </p>
  </section>

  <!-- Mode selector card -->
  <section class="mode-select-card" aria-label="工作模式选择">
    <div class="mode-select-header">
      <p class="section-kicker">工作模式</p>
      <h2>{{ routingMode === 'single' ? '单网卡代理模式' : '双网卡物理分流模式' }}</h2>
      <p>
        {{
          routingMode === 'single'
            ? '适用于常规单网卡 PC/笔记本：所有国内与局域网直连流量以及代理节点连接共用单张物理网卡，系统自动配置回环防护。'
            : '适用于严格物理隔离环境：内网/受限网走网卡 A，公网/代理走网卡 B。单卡掉线时严格物理熔断阻断，绝不混流。'
        }}
      </p>
    </div>
    <div class="mode-tabs">
      <button
        type="button"
        class="mode-tab-btn"
        :class="{ active: routingMode === 'single' }"
        :disabled="isRunning"
        @click="switchRoutingMode('single', isRunning)"
      >
        <strong>单网卡代理模式</strong>
        <small>仅需 1 块网卡 · 直连+代理</small>
      </button>
      <button
        type="button"
        class="mode-tab-btn"
        :class="{ active: routingMode === 'dual' }"
        :disabled="isRunning"
        @click="switchRoutingMode('dual', isRunning)"
      >
        <strong>双网卡物理分流模式</strong>
        <small>需 2 块物理网卡 · 安全隔离</small>
      </button>
    </div>
  </section>

  <section class="picker-grid" :class="{ 'single-picker': routingMode === 'single' }">
    <div class="picker">
      <label for="interface-a">
        <span class="route-letter">A</span>
        <span>
          <strong>{{ routingMode === 'single' ? '物理主网卡' : '出口 A' }}</strong>
          <small>{{ routingMode === 'single' ? '国内直连与代理底层' : '国内与局域网' }}</small>
        </span>
      </label>
      <select id="interface-a" v-model="selectedA">
        <option value="" disabled>选择网卡</option>
        <option
          v-for="item in candidates"
          :key="item.adapter.guid"
          :value="item.adapter.guid"
          :disabled="!item.eligible"
        >
          {{ item.adapter.friendly_name }} · {{ statusText(item.adapter.status) }}
        </option>
      </select>
    </div>
    <div v-if="routingMode === 'dual'" class="picker">
      <label for="interface-b">
        <span class="route-letter alternate">B</span>
        <span>
          <strong>出口 B</strong>
          <small>其他公网与代理底层</small>
        </span>
      </label>
      <select id="interface-b" v-model="selectedB">
        <option value="" disabled>选择网卡</option>
        <option
          v-for="item in candidates"
          :key="item.adapter.guid"
          :value="item.adapter.guid"
          :disabled="!item.eligible"
        >
          {{ item.adapter.friendly_name }} · {{ statusText(item.adapter.status) }}
        </option>
      </select>
    </div>
  </section>

  <p
    v-if="routingMode === 'dual' && sameAdapterWarning && selectedA && selectedA === selectedB"
    class="field-error"
    role="alert"
  >
    出口 A 和出口 B 不能绑定相同网卡，请为两个出口分别选择不同的物理网卡。
  </p>

  <section
    class="adapter-details"
    :class="{ 'single-adapter': routingMode === 'single' }"
    aria-label="所选网卡详情"
  >
    <article
      v-for="(adapter, role) in (routingMode === 'single' ? { '主网卡': selectedAdapterA } : { A: selectedAdapterA, B: selectedAdapterB })"
      :key="role"
    >
      <h3>{{ routingMode === 'single' ? '主网卡' : `出口 ${role}` }} · {{ adapter?.friendly_name ?? '未选择' }}</h3>
      <dl>
        <div>
          <dt>连接状态</dt>
          <dd>{{ statusText(adapter?.status ?? '未知') }}</dd>
        </div>
        <div>
          <dt>地址</dt>
          <dd>{{ adapter?.addresses?.map(a => `${a.ip}/${a.prefix_length}`).join('、') || '无' }}</dd>
        </div>
        <div>
          <dt>网关</dt>
          <dd>{{ formatList(adapter?.gateways) }}</dd>
        </div>
        <div>
          <dt>DNS</dt>
          <dd>{{ formatList(adapter?.dns_servers) }}</dd>
        </div>
      </dl>
    </article>
  </section>

  <section class="policy-strip">
    <div>
      <span>直连前缀</span>
      <strong>{{ directPrefixes.map(item => item.prefix).join('、') || '选择后生成' }}</strong>
    </div>
    <div>
      <span>DNS 策略</span>
      <strong>{{ routingMode === 'single' ? '国内直连经主网卡 / 代理经出口 C' : '国内经 A / 全球经 B，独立缓存' }}</strong>
    </div>
    <div>
      <span>IPv6 策略</span>
      <strong>阻止</strong>
    </div>
  </section>

  <section v-if="snapshot?.diagnostics?.length" class="diagnostics" aria-label="预检诊断">
    <h2>预检结果</h2>
    <div v-for="item in snapshot.diagnostics" :key="item.code" :class="['diagnostic', item.severity]">
      <strong>{{ item.severity === 'error' ? '错误' : '提示' }} · {{ item.code }}</strong>
      <span>{{ item.message }}</span>
      <small>{{ item.severity === 'error' ? '请恢复网卡连接或重新选择出口后重试。' : '保存后将按当前拓扑生成配置。' }}</small>
    </div>
  </section>

  <div class="actions">
    <button type="button" class="secondary" @click="setView('overview')">取消</button>
    <button
      type="button"
      class="primary"
      :disabled="busy || !selectedA || (routingMode === 'dual' && (!selectedB || selectedA === selectedB))"
      @click="saveSelection"
    >
      {{ busy ? '正在保存' : '保存并继续' }}
    </button>
  </div>
</template>
