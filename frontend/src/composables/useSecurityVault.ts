import { ref } from 'vue'
import {
  ChangeSecurityPIN,
  DisableSecurityPIN,
  EnableSecurityPIN,
  GetSecurityStatus,
  ListSubscriptions,
  UnlockSecurityVault,
} from '../../wailsjs/go/app/App'
import { notice } from './useFeedback'
import { refreshProxyState, subscriptionList } from './useProxyManager'

export const securityStatus = ref<{ pin_enabled: boolean; unlocked: boolean }>({
  pin_enabled: false,
  unlocked: true,
})

export const pinModalOpen = ref(false)
export const pinModalMode = ref<'unlock' | 'enable' | 'change' | 'disable'>('unlock')
export const pinInput = ref('')
export const oldPinInput = ref('')
export const pinModalError = ref('')
export const pinModalBusy = ref(false)

export async function loadSecurityStatus() {
  try {
    const s = await GetSecurityStatus()
    securityStatus.value = s
  } catch {}
}

export function openPinModal(mode: 'unlock' | 'enable' | 'change' | 'disable') {
  pinModalMode.value = mode
  pinInput.value = ''
  oldPinInput.value = ''
  pinModalError.value = ''
  pinModalOpen.value = true
}

export function closePinModal() {
  pinModalOpen.value = false
  pinInput.value = ''
  oldPinInput.value = ''
  pinModalError.value = ''
}

export async function submitPinModal() {
  pinModalError.value = ''
  pinModalBusy.value = true
  try {
    if (pinModalMode.value === 'unlock') {
      await UnlockSecurityVault(pinInput.value)
      notice.value = '安全保险箱已解锁'
    } else if (pinModalMode.value === 'enable') {
      await EnableSecurityPIN(pinInput.value)
      notice.value = '已启用 PIN 码安全保护'
    } else if (pinModalMode.value === 'change') {
      await ChangeSecurityPIN(oldPinInput.value, pinInput.value)
      notice.value = 'PIN 码已成功修改'
    } else if (pinModalMode.value === 'disable') {
      await DisableSecurityPIN(pinInput.value)
      notice.value = '已停用 PIN 码安全保护，恢复为本地便携密钥模式'
    }
    await loadSecurityStatus()
    await refreshProxyState()
    subscriptionList.value = await ListSubscriptions()
    closePinModal()
  } catch (err: any) {
    pinModalError.value = err?.message || String(err)
  } finally {
    pinModalBusy.value = false
  }
}
