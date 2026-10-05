<script setup lang="ts">
import { isRunning } from '../composables/useCoreManager'
import {
  chooseDNSPreset,
  dnsBusy,
  dnsPresets,
  dnsSettings,
  dnsTests,
  saveDNSSettings,
  setCustomDNS,
  testDNSServer,
} from '../composables/useDNSManager'
import {
  routingMode,
  selectedAdapterA,
  selectedAdapterB,
} from '../composables/useNetworkInterfaces'
import {
  chainNodes,
  chainSummaryText,
  isChainMode,
  isChainReady,
  proxyNodes,
} from '../composables/useProxyManager'
import {
  chooseSRSPreset,
  customRules,
  defaultOutbound,
  deleteCustomRule,
  deleteSRSSource,
  editCustomRule,
  editSRSSource,
  formatRuleAction,
  formatSRSUpdatedAt,
  masterRows,
  moveMasterRule,
  processRuleStatuses,
  refreshRulePreview,
  refreshSRSSource,
  resetRuleForm,
  ruleForm,
  ruleFormOpen,
  rulePreview,
  rulePreviewError,
  ruleTypeText,
  ruleUpdateOutbound,
  srsBusyID,
  srsForm,
  srsPresets,
  submitRule,
  toggleCustomRule,
  toggleSRSSource,
  updateDefaultOutbound,
  updateRuleUpdateOutbound,
} from '../composables/useRulesManager'
import type { RuleAction } from '../types'

async function handleSaveDNSSettings() {
  await saveDNSSettings(refreshRulePreview)
}
</script>

<template>
  <section class="rule-outlet-map" aria-label="规则出口映射">
    <article>
      <span>{{ routingMode === 'single' ? '物理主网卡' : '网卡 A' }}</span>
      <strong>{{ selectedAdapterA?.friendly_name ?? '尚未选择' }}</strong>
      <small>{{ selectedAdapterA?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterA?.gateways?.[0] ?? '无网关' }}</small>
    </article>
    <article v-if="routingMode === 'dual'">
      <span>网卡 B</span>
      <strong>{{ selectedAdapterB?.friendly_name ?? '尚未选择' }}</strong>
      <small>{{ selectedAdapterB?.addresses?.[0]?.ip ?? '无 IPv4 地址' }} · {{ selectedAdapterB?.gateways?.[0] ?? '无网关' }}</small>
    </article>
  </section>

  <section class="policy-strip" aria-label="当前 DNS 出口">
    <div>
      <span>{{ selectedAdapterA?.friendly_name ?? (routingMode === 'single' ? '主网卡' : '网卡 A') }} DNS</span>
      <strong>{{ dnsSettings.domestic.type.toUpperCase() }} · {{ dnsSettings.domestic.server }}:{{ dnsSettings.domestic.port }}</strong>
    </div>
    <div v-if="routingMode === 'dual'">
      <span>{{ selectedAdapterB?.friendly_name ?? '网卡 B' }} DNS</span>
      <strong>{{ dnsSettings.global.type.toUpperCase() }} · {{ dnsSettings.global.server }}:{{ dnsSettings.global.port }}</strong>
    </div>
    <div>
      <span>出口 C (代理) DNS</span>
      <strong>{{ (dnsSettings.proxy?.type ?? dnsSettings.global.type).toUpperCase() }} · {{ dnsSettings.proxy?.server ?? dnsSettings.global.server }}:{{ dnsSettings.proxy?.port ?? dnsSettings.global.port }}</strong>
    </div>
  </section>

  <section class="fallback-outbound" aria-label="未匹配流量兜底出口">
    <div>
      <strong>未匹配流量兜底出口</strong>
      <small>未命中自定义规则的流量将从此出口发出。</small>
    </div>
    <select
      :value="defaultOutbound"
      :disabled="isRunning"
      @change="updateDefaultOutbound(($event.target as HTMLSelectElement).value as 'a' | 'b' | 'c')"
    >
      <option value="a">
        {{ routingMode === 'single' ? '直连 (主网卡) · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '未选择' }}
      </option>
      <option v-if="routingMode === 'dual'" value="b">
        出口 B · {{ selectedAdapterB?.friendly_name ?? '未选择' }}
      </option>
      <option value="c">
        出口 C · 代理出站 ({{ isChainMode && isChainReady ? `套接: ${chainSummaryText}` : (proxyNodes.find(n => n.selected)?.name ?? '未配置') }})
      </option>
    </select>
  </section>

  <section class="fallback-outbound" aria-label="规则更新出口通道">
    <div>
      <strong>规则更新出口通道</strong>
      <small>拉取与更新 SRS 分流规则时使用的网络通道。</small>
    </div>
    <select
      :value="ruleUpdateOutbound"
      @change="updateRuleUpdateOutbound(($event.target as HTMLSelectElement).value as 'auto' | 'a' | 'b' | 'c')"
    >
      <option value="auto">自动选择 (代理就绪优先走代理，无代理走直连)</option>
      <option value="c">
        出口 C · 代理出站 ({{ isChainMode && isChainReady ? `套接: ${chainSummaryText}` : (proxyNodes.find(n => n.selected)?.name ?? '未配置') }})
      </option>
      <option v-if="routingMode === 'dual'" value="b">
        出口 B · {{ selectedAdapterB?.friendly_name ?? '网卡 B' }}
      </option>
      <option value="a">
        {{ routingMode === 'single' ? '主网卡直连 · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}
      </option>
    </select>
  </section>

  <section class="rules-heading">
    <div>
      <p class="section-kicker">按实际出口配置</p>
      <h2>用户覆盖规则</h2>
      <p>为 IP、域名、进程或 SRS 规则集指定出口；系统防环路和本地网络规则始终优先。</p>
    </div>
    <button class="primary" type="button" @click="ruleFormOpen ? resetRuleForm() : (ruleFormOpen = true)">
      {{ ruleFormOpen ? '取消' : '添加规则' }}
    </button>
  </section>

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
            {{ routingMode === 'single' ? '直连 (主网卡) · ' : '出口 A · ' }}{{ selectedAdapterA?.friendly_name ?? '网卡 A' }}
          </option>
          <option v-if="routingMode === 'dual'" value="b">
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

  <p v-if="customRules.some(rule => rule.type.startsWith('process-'))" class="process-note">
    进程名称可跨重启保持匹配；同名程序请使用完整路径。子进程不会隐式继承，请为实际联网的子进程单独添加规则。路径变化或无法识别时流量仍受基础域名/IP策略约束。
  </p>

  <section v-if="processRuleStatuses.length" class="process-statuses" aria-label="进程规则状态">
    <div
      v-for="status in processRuleStatuses"
      :key="`${status.type}-${status.value}`"
      :class="status.state"
    >
      <strong>{{ status.value }}</strong>
      <span>{{ status.message }}</span>
      <small v-if="status.paths?.length">{{ status.paths.join('、') }}</small>
    </div>
  </section>

  <section class="final-rules">
    <div class="rules-heading">
      <div>
        <p class="section-kicker">实际生成结果</p>
        <h2>最终规则顺序</h2>
        <p>包含基础设施、用户规则、直连前缀、内置规则及最终出口。</p>
      </div>
      <button class="secondary" type="button" @click="refreshRulePreview">刷新预览</button>
    </div>
    <p v-if="rulePreviewError" class="field-error">冲突诊断：{{ rulePreviewError }}</p>
    <div class="final-rule-list">
      <div v-for="item in rulePreview" :key="item.position">
        <span>{{ item.position }}</span>
        <strong>{{ item.category }}</strong>
        <code>{{ item.match.join('、') }}</code>
        <small>{{ item.action }}</small>
      </div>
    </div>
  </section>
</template>
