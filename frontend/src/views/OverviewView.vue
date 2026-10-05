<script setup lang="ts">
import { coreStatus, isRunning, startCore, stopCore } from '../composables/useCoreManager'
import { busy } from '../composables/useFeedback'
import { setView } from '../composables/useNavigation'
import {
  hasSavedSelection,
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
  setupRequired,
} from '../composables/useNetworkInterfaces'
import { formatRate, latestTraffic } from '../composables/useObservability'
import {
  chainNodes,
  chainSummaryText,
  chainTestResult,
  isChainMode,
  isChainReady,
  proxyNodes,
  proxyTests,
} from '../composables/useProxyManager'
import { defaultOutbound, updateDefaultOutbound } from '../composables/useRulesManager'
import { t } from '../i18n'

function statusText(status: string) {
  return status === 'up' ? t('status.up') : status === 'down' ? t('status.down') : status
}
</script>

<template>
  <section class="summary" aria-label="运行状态">
    <div>
      <span>{{ t('overview.selection') }}</span>
      <strong>{{ t(hasSavedSelection ? 'overview.completed' : 'overview.pending') }}</strong>
      <small>{{ routingMode === 'single' ? (selectedAdapterA?.friendly_name ?? '主网卡') : `${selectedAdapterA?.friendly_name ?? 'A'} / ${selectedAdapterB?.friendly_name ?? 'B'}` }}</small>
    </div>
    <div>
      <span>{{ t('overview.core') }}</span>
      <strong>{{ t(isRunning ? 'overview.applied' : 'overview.notApplied') }}</strong>
      <small>{{ isRunning ? `PID ${coreStatus.pid}` : t('overview.networkUnmanaged') }}</small>
    </div>
    <div>
      <span>{{ t('overview.verification') }}</span>
      <strong>{{ t(isRunning ? 'overview.healthy' : 'overview.unverified') }}</strong>
      <small>{{ routingMode === 'single' ? `流量 ${formatRate(latestTraffic.aDown + latestTraffic.aUp)}` : `A ${formatRate(latestTraffic.aDown + latestTraffic.aUp)} · B ${formatRate(latestTraffic.bDown + latestTraffic.bUp)}` }}</small>
    </div>
    <div>
      <span>{{ t('overview.ipv6') }}</span>
      <strong>{{ t('overview.blocked') }}</strong>
      <small>{{ t('overview.leakProtection') }}</small>
    </div>
  </section>

  <section v-if="setupRequired" class="setup-callout">
    <div>
      <p class="section-kicker">{{ t('overview.getStarted') }}</p>
      <h2>{{ t('overview.chooseOutlets') }}</h2>
      <p>{{ t('overview.outletDescription') }}</p>
    </div>
    <button class="primary" type="button" @click="setView('interfaces')">
      {{ t('overview.configure') }} <span aria-hidden="true">→</span>
    </button>
  </section>

  <template v-else>
    <section class="mode-row" aria-label="默认出口策略">
      <div>
        <p class="section-kicker">{{ t('overview.runMode') }}</p>
        <strong>
          {{
            defaultOutbound === 'a'
              ? routingMode === 'single'
                ? `直连 (${selectedAdapterA?.friendly_name ?? '主网卡'})`
                : `出口 A (${selectedAdapterA?.friendly_name ?? '网卡 A'})`
              : defaultOutbound === 'c'
              ? isChainMode && isChainReady
                ? `出口 C (链式套接: ${chainSummaryText})`
                : `出口 C (代理: ${proxyNodes.find(n => n.selected)?.name ?? '未选择'})`
              : `出口 B (${selectedAdapterB?.friendly_name ?? '网卡 B'})`
          }}
        </strong>
      </div>
      <div class="mode-switch">
        <button
          type="button"
          :class="{ active: defaultOutbound === 'a' }"
          :disabled="isRunning"
          @click="updateDefaultOutbound('a')"
        >
          {{ routingMode === 'single' ? '默认直连' : '默认 A' }}
        </button>
        <button
          v-if="routingMode === 'dual'"
          type="button"
          :class="{ active: defaultOutbound === 'b' }"
          :disabled="isRunning"
          @click="updateDefaultOutbound('b')"
        >
          默认 B
        </button>
        <button
          type="button"
          :class="{ active: defaultOutbound === 'c' }"
          :disabled="isRunning"
          @click="updateDefaultOutbound('c')"
        >
          默认 C (代理)
        </button>
      </div>
    </section>

    <section class="control-band">
      <div>
        <p class="section-kicker">{{ t('overview.coreControl') }}</p>
        <h2>{{ t(isRunning ? 'overview.running' : coreStatus.abnormal ? 'overview.interrupted' : 'overview.stopped') }}</h2>
        <p>{{ coreStatus.last_error || t(isRunning ? 'overview.runningDetail' : 'overview.stoppedDetail') }}</p>
      </div>
      <button v-if="isRunning" class="stop" type="button" :disabled="busy" @click="stopCore">
        <span aria-hidden="true">■</span>{{ t(busy ? 'state.stopping' : 'overview.stop') }}
      </button>
      <button v-else class="primary" type="button" :disabled="busy" @click="startCore">
        <span aria-hidden="true">▶</span>{{ t(busy ? 'overview.starting' : coreStatus.abnormal ? 'overview.restart' : 'overview.start') }}
      </button>
    </section>

    <section class="route-grid" aria-label="出口详情">
      <article
        v-for="(adapter, role) in (routingMode === 'single' ? { '主网卡': selectedAdapterA } : { A: selectedAdapterA, B: selectedAdapterB })"
        :key="role"
        class="route-row"
      >
        <span class="route-letter">{{ routingMode === 'single' ? '网卡' : role }}</span>
        <div>
          <h3>{{ adapter?.friendly_name }}</h3>
          <p>{{ routingMode === 'single' ? '主出网物理网卡（国内直连与代理底层出口）' : t(role === 'A' ? 'overview.domestic' : 'overview.public') }}</p>
        </div>
        <dl>
          <div>
            <dt>{{ t('overview.status') }}</dt>
            <dd>{{ statusText(adapter?.status ?? '') }}</dd>
          </div>
          <div>
            <dt>{{ t('overview.address') }}</dt>
            <dd>{{ adapter?.addresses?.[0]?.ip ?? t('common.none') }}</dd>
          </div>
          <div>
            <dt>{{ t('overview.gateway') }}</dt>
            <dd>{{ adapter?.gateways?.[0] ?? t('common.none') }}</dd>
          </div>
        </dl>
      </article>

      <article v-if="isChainMode && isChainReady && chainNodes[0] && chainNodes[chainNodes.length - 1]" class="route-row">
        <span class="route-letter alternate">C</span>
        <div>
          <h3>{{ chainSummaryText }}</h3>
          <p>链式套接代理出站（底层经出口 {{ chainNodes[0]?.egress?.toUpperCase() ?? 'B' }} 发起，终端落地于 {{ chainNodes[chainNodes.length - 1]?.name ?? '落地出口' }}）</p>
        </div>
        <dl>
          <div>
            <dt>模式</dt>
            <dd>链式套接 ({{ chainNodes.length }} 跳)</dd>
          </div>
          <div>
            <dt>前置跳板</dt>
            <dd>{{ chainNodes[0]?.name ?? '跳板' }} (出口 {{ chainNodes[0]?.egress?.toUpperCase() ?? 'B' }})</dd>
          </div>
          <div>
            <dt>落地出口</dt>
            <dd>{{ chainNodes[chainNodes.length - 1]?.name ?? '落地' }}</dd>
          </div>
          <div>
            <dt>整链状态</dt>
            <dd>{{ chainTestResult?.available ? `可用 · ${chainTestResult.latency_ms}ms` : '链路就绪' }}</dd>
          </div>
        </dl>
      </article>

      <article v-else-if="proxyNodes.find(n => n.selected)" class="route-row">
        <span class="route-letter alternate">C</span>
        <div>
          <h3>{{ proxyNodes.find(n => n.selected)?.name }}</h3>
          <p>代理出站（底层经{{ routingMode === 'single' ? '主网卡' : ('出口 ' + (proxyNodes.find(n => n.selected)?.egress === 'a' ? 'A' : 'B')) }}）</p>
        </div>
        <dl>
          <div>
            <dt>协议</dt>
            <dd>{{ proxyNodes.find(n => n.selected)?.type.toUpperCase() }}</dd>
          </div>
          <div>
            <dt>节点地址</dt>
            <dd>{{ proxyNodes.find(n => n.selected)?.server }}:{{ proxyNodes.find(n => n.selected)?.port }}</dd>
          </div>
          <div>
            <dt>状态</dt>
            <dd>{{ proxyTests[proxyNodes.find(n => n.selected)?.id || '']?.available ? '可用' : '活动节点' }}</dd>
          </div>
        </dl>
      </article>
    </section>
  </template>
</template>
