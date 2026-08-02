/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, never>, Record<string, never>, unknown>
  export default component
}

export interface ApplicationStatus {
  name: string
  version: string
  commit: string
  mode: string
  coreVersion: string
  ready: boolean
}

declare global {
  interface Window {
    go?: { main?: { App?: { GetStatus(): Promise<ApplicationStatus> } } }
  }
}
