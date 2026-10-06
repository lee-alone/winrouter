import type { nodes, subscriptions } from '../../wailsjs/go/models'

export interface ProxyProtocolMeta {
  value: 'http' | 'shadowsocks' | 'vmess' | 'vless' | 'trojan'
  label: string
  authKind: 'user-pass' | 'ss' | 'uuid' | 'trojan'
  supportsTLS: boolean
  forceTLS: boolean
  supportsTransport: boolean
  supportsFlow: boolean
  allowNoSecret: boolean
}

// Domain aliases for Wails proxy and subscription models
export type ProxyNode = nodes.Node
export type ProxyInput = nodes.Input
export type ProxySelection = nodes.ProxySelection
export type ProxyTestResult = nodes.TestResult
export type ProxyAuthenticationInput = nodes.AuthenticationInput
export type ProxyTLSInput = nodes.TLSInput
export type ProxyTransportInput = nodes.TransportInput

export type ProxySubscription = subscriptions.Subscription
export type ProxySubscriptionInput = subscriptions.Input

// Re-export underlying namespaces for seamless compatibility
export type { nodes, subscriptions }
