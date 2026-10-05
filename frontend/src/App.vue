<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'
import PinModal from './components/PinModal.vue'
import { cleanupApp, initializeApp } from './composables/useAppInit'
import {
  applicationConfigDirectory,
  applicationResetBusy,
  initializationFailed,
  repairApplicationSettings,
} from './composables/useAppSettings'
import {
  app,
  coreStatus,
  isRecovering,
  isRunning,
  pageTitle,
  recoveryStatus,
  stateLabel,
} from './composables/useCoreManager'
import { error, notice } from './composables/useFeedback'
import { currentView, setView } from './composables/useNavigation'
import { t } from './i18n'
import DiagnosticsView from './views/DiagnosticsView.vue'
import InterfacesView from './views/InterfacesView.vue'
import MonitoringView from './views/MonitoringView.vue'
import OverviewView from './views/OverviewView.vue'
import ProxyView from './views/ProxyView.vue'
import RulesView from './views/RulesView.vue'
import SettingsView from './views/SettingsView.vue'

onMounted(async () => {
  await initializeApp()
})

onUnmounted(() => {
  cleanupApp()
})
</script>

<template>
  <div class="shell">
    <aside>
      <div class="brand">
        <span class="brand-mark" aria-hidden="true">W</span>
        <span>WinRouter</span>
      </div>
      <nav :aria-label="t('nav.label')">
        <button :class="{ active: currentView === 'overview' }" type="button" @click="setView('overview')">
          <span aria-hidden="true">&#9636;</span>{{ t('nav.overview') }}
        </button>
        <button :class="{ active: currentView === 'monitoring' }" type="button" @click="setView('monitoring')">
          <span aria-hidden="true">&#9635;</span>{{ t('nav.monitoring') }}
        </button>
        <button :class="{ active: currentView === 'interfaces' }" type="button" @click="setView('interfaces')">
          <span aria-hidden="true">&#8644;</span>{{ t('nav.interfaces') }}
        </button>
        <button :class="{ active: currentView === 'rules' }" type="button" @click="setView('rules')">
          <span aria-hidden="true">&#8803;</span>{{ t('nav.rules') }}
        </button>
        <button :class="{ active: currentView === 'proxy' }" type="button" @click="setView('proxy')">
          <span aria-hidden="true">&#9673;</span>{{ t('nav.proxy') }}
        </button>
        <button :class="{ active: currentView === 'diagnostics' }" type="button" @click="setView('diagnostics')">
          <span aria-hidden="true">&#8801;</span>{{ t('nav.diagnostics') }}
        </button>
        <button :class="{ active: currentView === 'settings' }" type="button" @click="setView('settings')">
          <span aria-hidden="true">&#9881;</span>{{ t('nav.settings') }}
        </button>
      </nav>
      <div class="version">v{{ app.version }} · core {{ app.coreVersion }}</div>
    </aside>

    <main>
      <header>
        <div>
          <p class="eyebrow">{{ t('mode.direct') }}</p>
          <h1>{{ pageTitle }}</h1>
        </div>
        <span
          class="state"
          :class="{ running: isRunning && !isRecovering, danger: coreStatus.abnormal || recoveryStatus.state === 'failed' }"
        >
          <span aria-hidden="true">●</span>{{ stateLabel }}
        </span>
      </header>

      <div v-if="error" class="banner error" role="alert">
        <strong>{{ t('banner.failed') }}</strong>
        <span>{{ error }}</span>
        <button type="button" :aria-label="t('common.closeError')" @click="error = ''">×</button>
      </div>

      <section v-if="initializationFailed" class="setup-callout" aria-label="启动恢复模式">
        <div>
          <p class="section-kicker">恢复模式</p>
          <h2>应用配置未能加载</h2>
          <p>可备份并重建基础配置文件。代理节点、订阅和规则集来源不会被删除，修复后需要重启 WinRouter。</p>
          <code v-if="applicationConfigDirectory">{{ applicationConfigDirectory }}</code>
        </div>
        <button
          type="button"
          class="danger-button"
          :disabled="applicationResetBusy"
          @click="repairApplicationSettings"
        >
          {{ applicationResetBusy ? '正在修复' : '强制修复配置' }}
        </button>
      </section>

      <div v-if="notice" class="banner success" role="status">
        <strong>{{ t('banner.success') }}</strong>
        <span>{{ notice }}</span>
        <button type="button" :aria-label="t('common.closeNotice')" @click="notice = ''">×</button>
      </div>

      <OverviewView v-if="currentView === 'overview'" />
      <MonitoringView v-else-if="currentView === 'monitoring'" />
      <InterfacesView v-else-if="currentView === 'interfaces'" />
      <RulesView v-else-if="currentView === 'rules'" />
      <ProxyView v-else-if="currentView === 'proxy'" />
      <DiagnosticsView v-else-if="currentView === 'diagnostics'" />
      <SettingsView v-else-if="currentView === 'settings'" />
    </main>

    <PinModal />
  </div>
</template>
