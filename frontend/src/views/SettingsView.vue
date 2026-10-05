<script setup lang="ts">
import {
  applicationConfigDirectory,
  applicationResetBusy,
  autostartBusy,
  autostartEnabled,
  budgetBusy,
  budgetForm,
  copyApplicationConfigDirectory,
  isPortableConfig,
  networkResetBusy,
  networkResetResult,
  repairApplicationSettings,
  resetApplicationSettings,
  resetSavedInterfaces,
  resetTrafficBudget,
  resetWindowsNetworkStack,
  saveTrafficBudget,
  updateAutostart,
} from '../composables/useAppSettings'
import { isRunning } from '../composables/useCoreManager'
import {
  ipv6Busy,
  ipv6Policy,
  saveIPv6Policy,
} from '../composables/useNetworkInterfaces'
import { observations } from '../composables/useObservability'
import { refreshRulePreview } from '../composables/useRulesManager'
import { openPinModal, securityStatus } from '../composables/useSecurityVault'
import { locale, setLocale, t, type Locale } from '../i18n'

function changeLocale(event: Event) {
  setLocale((event.target as HTMLSelectElement).value as Locale)
}

async function handleSaveIPv6Policy() {
  await saveIPv6Policy(refreshRulePreview)
}
</script>

<template>
  <section class="settings-list" aria-label="IPv6 分流设置">
    <div class="setting-row">
      <div>
        <h2>IPv6 分流</h2>
        <p>关闭时拒绝 AAAA 查询和 IPv6 流量；开启前必须通过两个出口的 IPv6 地址、默认路由与前缀重叠预检。</p>
      </div>
      <div class="budget-controls">
        <label class="toggle">
          <input
            v-model="ipv6Policy"
            type="checkbox"
            true-value="split"
            false-value="block"
            :disabled="ipv6Busy || isRunning"
          >
          <span>{{ ipv6Policy === 'split' ? '已启用' : '已关闭' }}</span>
        </label>
        <button
          type="button"
          class="primary"
          :disabled="ipv6Busy || isRunning"
          @click="handleSaveIPv6Policy"
        >
          {{ ipv6Busy ? '正在预检' : '保存 IPv6 策略' }}
        </button>
      </div>
    </div>
  </section>

  <section class="settings-list" :aria-label="t('settings.language')">
    <div class="setting-row">
      <div>
        <h2>{{ t('settings.language') }}</h2>
        <p>{{ t('settings.languageDetail') }}</p>
      </div>
      <select class="language-select" :value="locale" @change="changeLocale">
        <option value="zh-CN">简体中文</option>
        <option value="en-US">English</option>
      </select>
    </div>
  </section>

  <section class="settings-list" aria-label="流量预算设置">
    <div class="setting-row budget-setting">
      <div>
        <h2>{{ t('settings.budget') }}</h2>
        <p>{{ t('settings.budgetDetail') }}</p>
      </div>
      <div class="budget-controls">
        <label class="toggle">
          <input v-model="budgetForm.enabled" type="checkbox" :disabled="budgetBusy">
          <span>{{ t(budgetForm.enabled ? 'common.enabled' : 'common.disabled') }}</span>
        </label>
        <label>
          {{ t('settings.budgetGB') }}
          <input
            v-model.number="budgetForm.budget_gb"
            type="number"
            min="0.1"
            max="100000"
            step="0.1"
            :disabled="budgetBusy"
          >
        </label>
        <label>
          {{ t('settings.warning') }}
          <input
            v-model.number="budgetForm.warning_percent"
            type="number"
            min="1"
            max="100"
            step="1"
            :disabled="budgetBusy"
          >
        </label>
        <button type="button" class="primary" :disabled="budgetBusy" @click="saveTrafficBudget">
          {{ t('common.save') }}
        </button>
        <button
          type="button"
          class="secondary"
          :disabled="budgetBusy || !observations.traffic_budget.used_bytes"
          @click="resetTrafficBudget"
        >
          {{ t('settings.resetMonth') }}
        </button>
      </div>
    </div>
  </section>

  <section class="settings-list" aria-label="应用设置">
    <div class="setting-row">
      <div>
        <h2>{{ t('settings.autostart') }}</h2>
        <p>{{ t('settings.autostartDetail') }}</p>
      </div>
      <label class="toggle">
        <input
          v-model="autostartEnabled"
          type="checkbox"
          :disabled="autostartBusy"
          @change="updateAutostart"
        >
        <span>{{ t(autostartEnabled ? 'common.enabled' : 'common.disabled') }}</span>
      </label>
    </div>
  </section>

  <section class="settings-list" aria-label="数据安全与凭据保护">
    <div class="setting-row">
      <div>
        <p class="section-kicker">安全保护</p>
        <h2>节点凭据安全锁 (PIN 码保险箱)</h2>
        <p>
          采用 AES-256-GCM 与 PBKDF2 高强度派生加密。当前状态：<strong>{{
            securityStatus.pin_enabled
              ? securityStatus.unlocked
                ? '已启用 PIN 码保护 (已解锁)'
                : '已锁定，需要输入 PIN 码'
              : '便携免密模式 (基于本地密钥，支持跨设备移动)'
          }}</strong>
        </p>
      </div>
      <div class="proxy-actions">
        <button
          v-if="securityStatus.pin_enabled && !securityStatus.unlocked"
          type="button"
          class="primary-button"
          @click="openPinModal('unlock')"
        >
          输入 PIN 解锁
        </button>
        <button
          v-if="!securityStatus.pin_enabled"
          type="button"
          class="secondary"
          @click="openPinModal('enable')"
        >
          启用 PIN 码保护
        </button>
        <button
          v-if="securityStatus.pin_enabled && securityStatus.unlocked"
          type="button"
          class="secondary"
          @click="openPinModal('change')"
        >
          修改 PIN 码
        </button>
        <button
          v-if="securityStatus.pin_enabled && securityStatus.unlocked"
          type="button"
          class="danger-button"
          @click="openPinModal('disable')"
        >
          停用 PIN 保护
        </button>
      </div>
    </div>
  </section>

  <section class="settings-list network-reset-section" aria-label="应用故障恢复">
    <div class="setting-row network-reset-setting">
      <div>
        <p class="section-kicker">故障恢复</p>
        <h2>应用配置恢复</h2>
        <p>仅重置网卡选择，或在自动备份后恢复规则、DNS、IPv6 策略和网卡选择默认值。不会删除代理节点、订阅、规则集来源，也不会修改 Windows 网络组件。</p>
        <div class="config-directory">
          <span>当前配置目录 {{ isPortableConfig ? '(便携模式)' : '(系统模式)' }}</span>
          <code>{{ applicationConfigDirectory || '无法确定配置目录' }}</code>
          <small>{{ isPortableConfig ? '配置文件保存在程序所在目录下的 config 文件夹中，方便整体迁移与备份。' : '当前处于受限目录，配置文件保存在系统用户配置目录中。' }}</small>
        </div>
      </div>
      <div class="proxy-actions">
        <button
          type="button"
          class="secondary"
          :disabled="applicationResetBusy"
          @click="resetSavedInterfaces"
        >
          重置网卡选择
        </button>
        <button
          type="button"
          class="secondary"
          :disabled="applicationResetBusy"
          @click="resetApplicationSettings"
        >
          恢复应用默认设置
        </button>
        <button
          type="button"
          class="danger-button"
          :disabled="applicationResetBusy"
          @click="repairApplicationSettings"
        >
          {{ applicationResetBusy ? '正在处理' : '强制修复配置' }}
        </button>
      </div>
    </div>
    <div class="setting-row">
      <div>
        <h2>配置文件位置 {{ isPortableConfig ? '(程序便携目录)' : '(系统用户目录)' }}</h2>
        <p>{{ isPortableConfig ? '规则、网卡选择、DNS、节点与备份均保存在程序同级 config 目录，支持解压即用和随时拷贝。' : '规则、网卡选择、DNS、代理节点、订阅、缓存和恢复备份均保存在当前用户目录。' }}</p>
      </div>
      <button
        type="button"
        class="secondary"
        :disabled="!applicationConfigDirectory"
        @click="copyApplicationConfigDirectory"
      >
        复制路径
      </button>
    </div>
  </section>

  <section class="settings-list network-reset-section" aria-label="高级网络修复">
    <div class="setting-row network-reset-setting">
      <div>
        <p class="section-kicker">高级修复</p>
        <h2>重置 Windows 网络组件</h2>
        <p class="danger-copy">此操作会停止分流核心，并重置 Winsock、TCP/IP 与 DNS 缓存。可能清除静态 IP、网关、DNS、VPN 或虚拟网卡配置，导致网络中断；完成后通常需要重启 Windows。</p>
      </div>
      <button
        type="button"
        class="danger-button"
        :disabled="networkResetBusy"
        @click="resetWindowsNetworkStack"
      >
        {{ networkResetBusy ? '正在重置' : '重置网络组件' }}
      </button>
    </div>
    <div v-if="networkResetResult" class="network-reset-result" :class="{ failed: !networkResetResult.success }">
      <strong>{{ networkResetResult.success ? '全部步骤已完成' : '部分步骤执行失败' }} · {{ networkResetResult.restart_required ? '需要重启 Windows' : '无需重启' }}</strong>
      <div v-for="step in networkResetResult.steps" :key="step.command" class="network-reset-step">
        <span :class="step.success ? 'step-success' : 'step-failed'">{{ step.success ? '成功' : `失败 (${step.exit_code})` }}</span>
        <code>{{ step.command }}</code>
        <pre v-if="step.output">{{ step.output }}</pre>
      </div>
    </div>
  </section>
</template>
