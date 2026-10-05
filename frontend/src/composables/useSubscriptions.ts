import { ref } from 'vue'
import {
  AddSubscription,
  DeleteSubscription,
  ListSubscriptions,
  RefreshSubscription,
  UpdateSubscription,
} from '../../wailsjs/go/app/App'
import type { subscriptions } from '../../wailsjs/go/models'
import { busy, error, messageOf, notice } from './useFeedback'
import { refreshProxyState } from './useProxyNodes'

export const subscriptionList = ref<subscriptions.Subscription[]>([])
export const subscriptionFormOpen = ref(false)
export const subscriptionForm = ref<{ id: string; name: string; url: string; egress: 'a' | 'b' }>({
  id: '',
  name: '',
  url: '',
  egress: 'b',
})
export const refreshingSubscriptionID = ref('')

export function resetSubscriptionForm(): void {
  subscriptionForm.value = { id: '', name: '', url: '', egress: 'b' }
  subscriptionFormOpen.value = false
}

export function editSubscription(item: subscriptions.Subscription): void {
  subscriptionForm.value = { id: item.id, name: item.name, url: '', egress: (item.egress as any) || 'b' }
  subscriptionFormOpen.value = true
}

export async function refreshSubscriptions(): Promise<void> {
  try {
    subscriptionList.value = await ListSubscriptions()
  } catch (reason) {
    console.error('Failed to list subscriptions:', reason)
  }
}

export async function saveSubscription(): Promise<void> {
  busy.value = true
  error.value = ''
  try {
    if (subscriptionForm.value.id) await UpdateSubscription(subscriptionForm.value)
    else await AddSubscription(subscriptionForm.value)
    subscriptionList.value = await ListSubscriptions()
    resetSubscriptionForm()
    await refreshProxyState()
    notice.value = '订阅定义已安全保存。'
  } catch (reason) {
    error.value = `无法保存订阅：${messageOf(reason)}`
  } finally {
    busy.value = false
  }
}

export async function refreshSubscription(item: subscriptions.Subscription): Promise<void> {
  refreshingSubscriptionID.value = item.id
  error.value = ''
  try {
    await RefreshSubscription(item.id)
    subscriptionList.value = await ListSubscriptions()
    await refreshProxyState()
    notice.value = '订阅已校验并原子更新。'
  } catch (reason) {
    subscriptionList.value = await ListSubscriptions()
    error.value = `订阅更新失败，已保留原节点：${messageOf(reason)}`
  } finally {
    refreshingSubscriptionID.value = ''
  }
}

export async function deleteSubscription(item: subscriptions.Subscription): Promise<void> {
  if (!window.confirm(`删除订阅“${item.name}”及其节点？`)) return
  try {
    await DeleteSubscription(item.id)
    subscriptionList.value = await ListSubscriptions()
    await refreshProxyState()
    notice.value = '订阅及其节点已删除。'
  } catch (reason) {
    error.value = `无法删除订阅：${messageOf(reason)}`
  }
}
