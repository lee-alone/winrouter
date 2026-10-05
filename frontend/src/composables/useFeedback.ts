import { ref } from 'vue'

export const notice = ref('')
export const error = ref('')
export const busy = ref(false)

export function messageOf(reason: unknown): string {
  return reason instanceof Error ? reason.message : String(reason)
}

export function clearFeedback() {
  notice.value = ''
  error.value = ''
}
