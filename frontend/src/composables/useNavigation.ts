import { ref } from 'vue'
import type { View } from '../types'

export const currentView = ref<View>('overview')

export function setView(view: View) {
  currentView.value = view
}
