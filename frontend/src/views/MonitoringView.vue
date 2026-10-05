<script setup lang="ts">
import { coreStatus } from '../composables/useCoreManager'
import {
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
} from '../composables/useNetworkInterfaces'
import {
  counterA,
  counterB,
  displayedUsage,
  formatBytes,
  formatRate,
  latestTraffic,
  observations,
  observationsError,
  resetInterfaceUsage,
  ruleHitA,
  ruleHitB,
  toggleConnectionObservation,
  traffic,
  trafficHeight,
} from '../composables/useObservability'
import { t } from '../i18n'
</script>

<template>
  <section class="monitor-summary" aria-label="连接与规则摘要">
    <div>
      <span>{{ t('monitor.activeTcp') }}</span>
      <strong>{{ observations.connections.active_tcp }}</strong>
      <small>{{ observations.connections.established_tcp }} {{ t('monitor.established') }} · {{ observations.connections.listening_tcp }} {{ t('monitor.listening') }}</small>
    </div>
    <div>
      <span>{{ t('monitor.udp') }}</span>
      <strong>{{ observations.connections.udp_endpoints }}</strong>
      <small>{{ t('monitor.systemSnapshot') }}</small>
    </div>
    <div>
      <span>{{ routingMode === 'single' ? '直连命中' : t('monitor.hitA') }}</span>
      <strong>{{ ruleHitA }}</strong>
      <small>{{ t('monitor.logWindow') }}</small>
    </div>
    <div v-if="routingMode === 'dual'">
      <span>{{ t('monitor.hitB') }}</span>
      <strong>{{ ruleHitB }}</strong>
      <small>{{ t('monitor.logWindow') }}</small>
    </div>
  </section>

  <p v-if="observationsError" class="observation-error">
    监控采样失败：{{ observationsError }}。当前显示上一次可用数据。
  </p>

  <section
    v-if="observations.traffic_budget.enabled"
    :class="['budget-status', { warning: observations.traffic_budget.warning_reached }]"
    aria-live="polite"
  >
    <div>
      <p class="section-kicker">{{ t('monitor.budget') }}</p>
      <h2>{{ formatBytes(observations.traffic_budget.used_bytes) }} / {{ observations.traffic_budget.budget_gb }} GB</h2>
      <p>{{ t('monitor.budgetScope') }}</p>
    </div>
    <strong>{{ observations.traffic_budget.used_percent.toFixed(1) }}%</strong>
  </section>

  <section class="traffic-section">
    <div class="traffic-section-heading">
      <div>
        <p class="section-kicker">{{ t('monitor.interfaceTraffic') }}</p>
        <h2>各网卡实时流量</h2>
      </div>
      <strong>仅供参考</strong>
    </div>
    <p class="traffic-disclaimer">
      数据来自 Windows 物理网卡计数器，包含该网卡上其他应用的流量；“累计清零”仅重设本机显示基线，不修改系统计数器。数值仅供参考，可能在系统重启、驱动重置或网卡重连后变化，不等同于 WinRouter 分流量或运营商账单。
    </p>
    <div class="traffic-panels">
      <article
        v-for="role in (routingMode === 'single' ? (['A'] as const) : (['A', 'B'] as const))"
        :key="role"
        class="traffic-panel"
      >
        <div class="traffic-heading">
          <div>
            <span>{{ routingMode === 'single' ? '物理主网卡' : `网卡 ${role}` }}</span>
            <h3>{{ role === 'A' ? (selectedAdapterA?.friendly_name ?? '未选择') : (selectedAdapterB?.friendly_name ?? '未选择') }}</h3>
          </div>
          <div class="traffic-heading-actions">
            <small>{{ routingMode === 'single' ? '直连与代理底层出口' : (role === 'A' ? '国内与局域网出口' : '其他公网出口') }}</small>
            <button
              type="button"
              class="secondary"
              :disabled="!(role === 'A' ? counterA : counterB)"
              @click="resetInterfaceUsage(role)"
            >
              累计清零
            </button>
          </div>
        </div>
        <div class="traffic-totals">
          <div>
            <span>清零后累计下载</span>
            <strong>{{ formatBytes(displayedUsage(role === 'A' ? counterA : counterB, 'received')) }}</strong>
          </div>
          <div>
            <span>清零后累计上传</span>
            <strong>{{ formatBytes(displayedUsage(role === 'A' ? counterA : counterB, 'transmitted')) }}</strong>
          </div>
        </div>
        <div class="traffic-legend">
          <span :class="role === 'A' ? 'a-down' : 'b-down'">下载 {{ formatRate(role === 'A' ? latestTraffic.aDown : latestTraffic.bDown) }}</span>
          <span :class="role === 'A' ? 'a-up' : 'b-up'">上传 {{ formatRate(role === 'A' ? latestTraffic.aUp : latestTraffic.bUp) }}</span>
        </div>
        <div class="traffic-chart" :aria-label="`网卡 ${role} 流量速率趋势`">
          <div v-if="!traffic.length" class="chart-empty">{{ t('monitor.baseline') }}</div>
          <div v-for="point in traffic" :key="point.at" class="traffic-column">
            <template v-if="role === 'A'">
              <i class="a-down" :style="{ height: trafficHeight(point.aDown, 'A') }"></i>
              <i class="a-up" :style="{ height: trafficHeight(point.aUp, 'A') }"></i>
            </template>
            <template v-else>
              <i class="b-down" :style="{ height: trafficHeight(point.bDown, 'B') }"></i>
              <i class="b-up" :style="{ height: trafficHeight(point.bUp, 'B') }"></i>
            </template>
          </div>
        </div>
        <div class="traffic-axis">
          <span>{{ t('monitor.earlier') }}</span>
          <span>{{ t('monitor.now') }}</span>
        </div>
      </article>
    </div>
  </section>

  <section class="traffic-section connection-observation">
    <div class="traffic-section-heading">
      <div>
        <p class="section-kicker">连接观测</p>
        <h2>域名与出口</h2>
      </div>
      <button
        type="button"
        class="secondary"
        :disabled="coreStatus.state === 'running'"
        @click="toggleConnectionObservation"
      >
        {{ observations.connection_observation ? '关闭观测' : '开启观测' }}
      </button>
    </div>
    <p class="traffic-disclaimer">仅在开启后读取本机 sing-box 连接 API；切换前需停止核心。</p>
    <div v-if="observations.connection_observation && observations.connection_events.length" class="connection-table">
      <div class="connection-row connection-head">
        <span>域名 / IP</span><span>类型</span><span>规则</span><span>出口</span><span>连接</span><span>流量</span>
      </div>
      <div
        v-for="item in observations.connection_events"
        :key="`${item.domain}-${item.ip}-${item.outbound}`"
        class="connection-row"
      >
        <span>
          <strong>{{ item.domain || item.ip || '未知' }}</strong>
          <small v-if="item.domain && item.ip && item.domain !== item.ip">{{ item.ip }}</small>
        </span>
        <span>{{ item.address_type || '-' }} · {{ item.protocol || '-' }}</span>
        <span>{{ item.rule || 'final' }}</span>
        <span>{{ item.outbound || '未知' }}</span>
        <span>{{ item.established }} / {{ item.attempts }}</span>
        <span>{{ formatBytes(item.bytes_down) }} ↓ · {{ formatBytes(item.bytes_up) }} ↑</span>
      </div>
    </div>
    <p v-else-if="observations.connection_observation" class="traffic-disclaimer">
      尚无活动连接；核心启动后每 5 秒刷新。
    </p>
  </section>

  <section class="monitor-notes">
    <div>
      <strong>{{ t('monitor.ruleScope') }}</strong>
      <span>{{ t('monitor.ruleScopeDetail') }}</span>
    </div>
    <div>
      <strong>{{ t('monitor.connectionScope') }}</strong>
      <span>{{ t('monitor.connectionScopeDetail') }}</span>
    </div>
  </section>
</template>
