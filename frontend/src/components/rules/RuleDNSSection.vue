<script setup lang="ts">
import { isRunning } from '../../composables/useCoreManager'
import {
  chooseDNSPreset,
  dnsBusy,
  dnsPresets,
  dnsSettings,
  dnsTests,
  saveDNSSettings,
  setCustomDNS,
  testDNSServer,
} from '../../composables/useDNSManager'
import {
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
} from '../../composables/useNetworkInterfaces'
import {
  chainNodes,
  isChainMode,
  isChainReady,
  proxyNodes,
} from '../../composables/useProxyManager'
import { refreshRulePreview } from '../../composables/useRulePreview'

async function handleSaveDNSSettings() {
  await saveDNSSettings(refreshRulePreview)
}
</script>

<template>
  <section class="outlet-rule-columns" aria-label="出口 DNS 设置">
    <article class="outlet-rule-group">
      <header>
        <span>{{ routingMode === 'single' ? '主网卡直连 DNS' : '出口 A DNS' }}</span>
        <h3>{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}</h3>
        <small>管理该出口使用的域名解析服务</small>
      </header>
      <details class="outlet-dns-settings">
        <summary class="outlet-dns-heading">
          <strong>{{ routingMode === 'single' ? '主网卡直连 DNS' : '出口 A DNS' }}</strong>
          <small>{{ dnsSettings.domestic.type.toUpperCase() }} · {{ dnsSettings.domestic.server }}:{{ dnsSettings.domestic.port }}</small>
        </summary>
        <div class="dns-row">
          <label>
            DNS 预设
            <select
              v-model="dnsSettings.domestic.preset_id"
              :disabled="dnsBusy"
              @change="dnsSettings.domestic.preset_id ? chooseDNSPreset('domestic') : setCustomDNS('domestic')"
            >
              <option value="">自定义</option>
              <option
                v-for="preset in dnsPresets.filter(item => item.scope === 'domestic')"
                :key="preset.id"
                :value="preset.id"
              >
                {{ preset.name }}
              </option>
            </select>
          </label>
          <label>
            协议
            <select
              v-model="dnsSettings.domestic.type"
              :disabled="dnsBusy || Boolean(dnsSettings.domestic.preset_id)"
              @change="setCustomDNS('domestic')"
            >
              <option value="udp">UDP</option>
              <option value="tls">DoT</option>
              <option value="https">DoH</option>
            </select>
          </label>
          <label>
            服务器 IP
            <input
              v-model.trim="dnsSettings.domestic.server"
              required
              :readonly="Boolean(dnsSettings.domestic.preset_id)"
              placeholder="1.1.1.1"
              @input="setCustomDNS('domestic')"
            >
          </label>
          <label>
            端口
            <input
              v-model.number="dnsSettings.domestic.port"
              type="number"
              min="1"
              max="65535"
              required
              :readonly="Boolean(dnsSettings.domestic.preset_id)"
              @input="setCustomDNS('domestic')"
            >
          </label>
          <label :class="{ 'dns-field-placeholder': dnsSettings.domestic.type === 'udp' }">
            TLS 域名
            <input
              v-model.trim="dnsSettings.domestic.server_name"
              :required="dnsSettings.domestic.type !== 'udp'"
              :disabled="dnsSettings.domestic.type === 'udp'"
              :readonly="Boolean(dnsSettings.domestic.preset_id)"
              :placeholder="dnsSettings.domestic.type === 'udp' ? 'UDP 不需要' : 'dns.example.com'"
              @input="setCustomDNS('domestic')"
            >
          </label>
          <button
            type="button"
            class="dns-test"
            :class="dnsTests.domestic"
            :disabled="dnsTests.domestic === 'testing' || dnsBusy || isRunning"
            @click="testDNSServer('domestic')"
          >
            {{ dnsTests.domestic === 'testing' ? '测试中' : dnsTests.domestic === 'success' ? '成功' : dnsTests.domestic === 'failed' ? '失败' : '测试' }}
          </button>
        </div>
        <div class="dns-actions">
          <button
            type="button"
            class="primary"
            :disabled="dnsBusy || isRunning"
            @click="handleSaveDNSSettings"
          >
            {{ dnsBusy ? '正在校验' : (routingMode === 'single' ? '保存直连 DNS' : '保存 A DNS') }}
          </button>
        </div>
      </details>
    </article>

    <article v-if="routingMode === 'dual'" class="outlet-rule-group">
      <header>
        <span>出口 B DNS</span>
        <h3>{{ selectedAdapterB?.friendly_name ?? '网卡 B' }}</h3>
        <small>管理该出口使用的域名解析服务</small>
      </header>
      <details class="outlet-dns-settings">
        <summary class="outlet-dns-heading">
          <strong>出口 B DNS</strong>
          <small>{{ dnsSettings.global.type.toUpperCase() }} · {{ dnsSettings.global.server }}:{{ dnsSettings.global.port }}</small>
        </summary>
        <div class="dns-row">
          <label>
            DNS 预设
            <select
              v-model="dnsSettings.global.preset_id"
              :disabled="dnsBusy"
              @change="dnsSettings.global.preset_id ? chooseDNSPreset('global') : setCustomDNS('global')"
            >
              <option value="">自定义</option>
              <optgroup label="国内加密 DNS (推荐)">
                <option
                  v-for="preset in dnsPresets.filter(item => item.scope === 'domestic')"
                  :key="preset.id"
                  :value="preset.id"
                >
                  {{ preset.name }}
                </option>
              </optgroup>
              <optgroup label="国际加密 DNS">
                <option
                  v-for="preset in dnsPresets.filter(item => item.scope === 'global')"
                  :key="preset.id"
                  :value="preset.id"
                >
                  {{ preset.name }}
                </option>
              </optgroup>
            </select>
          </label>
          <label>
            协议
            <select
              v-model="dnsSettings.global.type"
              :disabled="dnsBusy || Boolean(dnsSettings.global.preset_id)"
              @change="setCustomDNS('global')"
            >
              <option value="udp">UDP</option>
              <option value="tls">DoT</option>
              <option value="https">DoH</option>
            </select>
          </label>
          <label>
            服务器 IP
            <input
              v-model.trim="dnsSettings.global.server"
              required
              :readonly="Boolean(dnsSettings.global.preset_id)"
              placeholder="1.1.1.1"
              @input="setCustomDNS('global')"
            >
          </label>
          <label>
            端口
            <input
              v-model.number="dnsSettings.global.port"
              type="number"
              min="1"
              max="65535"
              required
              :readonly="Boolean(dnsSettings.global.preset_id)"
              @input="setCustomDNS('global')"
            >
          </label>
          <label :class="{ 'dns-field-placeholder': dnsSettings.global.type === 'udp' }">
            TLS 域名
            <input
              v-model.trim="dnsSettings.global.server_name"
              :required="dnsSettings.global.type !== 'udp'"
              :disabled="dnsSettings.global.type === 'udp'"
              :readonly="Boolean(dnsSettings.global.preset_id)"
              :placeholder="dnsSettings.global.type === 'udp' ? 'UDP 不需要' : 'dns.example.com'"
              @input="setCustomDNS('global')"
            >
          </label>
          <button
            type="button"
            class="dns-test"
            :class="dnsTests.global"
            :disabled="dnsTests.global === 'testing' || dnsBusy || isRunning"
            @click="testDNSServer('global')"
          >
            {{ dnsTests.global === 'testing' ? '测试中' : dnsTests.global === 'success' ? '成功' : dnsTests.global === 'failed' ? '失败' : '测试' }}
          </button>
        </div>
        <div class="dns-actions">
          <button
            type="button"
            class="primary"
            :disabled="dnsBusy || isRunning"
            @click="handleSaveDNSSettings"
          >
            {{ dnsBusy ? '正在校验' : '保存 B DNS' }}
          </button>
        </div>
      </details>
    </article>

    <article class="outlet-rule-group">
      <header>
        <span>出口 C DNS</span>
        <h3>{{ isChainMode && isChainReady && chainNodes.length ? `套接落地: ${chainNodes[chainNodes.length - 1]?.name || '落地节点'}` : (proxyNodes.find(n => n.selected)?.name ?? '代理出站') }}</h3>
        <small>管理该代理出口使用的域名解析服务</small>
      </header>
      <details class="outlet-dns-settings">
        <summary class="outlet-dns-heading">
          <strong>出口 C DNS</strong>
          <small>{{ (dnsSettings.proxy?.type ?? 'udp').toUpperCase() }} · {{ dnsSettings.proxy?.server ?? '8.8.8.8' }}:{{ dnsSettings.proxy?.port ?? 53 }}</small>
        </summary>
        <div v-if="dnsSettings.proxy" class="dns-row">
          <label>
            DNS 预设
            <select
              v-model="dnsSettings.proxy.preset_id"
              :disabled="dnsBusy"
              @change="dnsSettings.proxy.preset_id ? chooseDNSPreset('proxy') : setCustomDNS('proxy')"
            >
              <option value="">自定义</option>
              <option
                v-for="preset in dnsPresets.filter(item => item.scope === 'proxy')"
                :key="preset.id"
                :value="preset.id"
              >
                {{ preset.name }}
              </option>
            </select>
          </label>
          <label>
            协议
            <select
              v-model="dnsSettings.proxy.type"
              :disabled="dnsBusy || Boolean(dnsSettings.proxy.preset_id)"
              @change="setCustomDNS('proxy')"
            >
              <option value="udp">UDP</option>
              <option value="tls">DoT</option>
              <option value="https">DoH</option>
            </select>
          </label>
          <label>
            服务器 IP
            <input
              v-model.trim="dnsSettings.proxy.server"
              required
              :readonly="Boolean(dnsSettings.proxy.preset_id)"
              placeholder="8.8.8.8"
              @input="setCustomDNS('proxy')"
            >
          </label>
          <label>
            端口
            <input
              v-model.number="dnsSettings.proxy.port"
              type="number"
              min="1"
              max="65535"
              required
              :readonly="Boolean(dnsSettings.proxy.preset_id)"
              @input="setCustomDNS('proxy')"
            >
          </label>
          <label :class="{ 'dns-field-placeholder': dnsSettings.proxy.type === 'udp' }">
            TLS 域名
            <input
              v-model.trim="dnsSettings.proxy.server_name"
              :required="dnsSettings.proxy.type !== 'udp'"
              :disabled="dnsSettings.proxy.type === 'udp'"
              :readonly="Boolean(dnsSettings.proxy.preset_id)"
              :placeholder="dnsSettings.proxy.type === 'udp' ? 'UDP 不需要' : 'dns.example.com'"
              @input="setCustomDNS('proxy')"
            >
          </label>
          <button
            type="button"
            class="dns-test"
            :class="dnsTests.proxy"
            :disabled="dnsTests.proxy === 'testing' || dnsBusy || isRunning"
            @click="testDNSServer('proxy')"
          >
            {{ dnsTests.proxy === 'testing' ? '测试中' : dnsTests.proxy === 'success' ? '成功' : dnsTests.proxy === 'failed' ? '失败' : '测试' }}
          </button>
        </div>
        <div class="dns-actions">
          <button
            type="button"
            class="primary"
            :disabled="dnsBusy || isRunning"
            @click="handleSaveDNSSettings"
          >
            {{ dnsBusy ? '正在校验' : '保存 C DNS' }}
          </button>
        </div>
      </details>
    </article>
  </section>
</template>
