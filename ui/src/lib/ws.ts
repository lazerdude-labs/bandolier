import { useEffect, useRef, useState } from 'react'
import { fetchWSToken } from './api'

export interface DeploymentEvent {
  type: 'step_start' | 'step_end' | 'step_progress' | 'log' | 'ansible_event' | 'deployment_complete'
  step?: string
  stream?: string
  text?: string
  status?: string
  exit_code?: number
  data?: unknown
  ts: string
}

// StepProgressData is the shape of `data` on step_progress events emitted by
// the bundle install path. The UI uses these to render a sticky status banner
// above the log stream — helm's stdout goes silent for minutes during --wait,
// so this is what the user actually sees changing during a multi-chart install.
export type StepProgressData =
  | { phase: 'bundle_start'; bundle: string; total: number }
  | { phase: 'chart_install'; chart: string; release: string; namespace: string; index: number; total: number }
  | { phase: 'rollback'; failed_chart: string; rollback_count: number }

export type LogStreamReturn = {
  events: DeploymentEvent[]
  reconnectIn: number | null
}

function useReconnectingWS(urlPath: string): LogStreamReturn {
  const [events, setEvents] = useState<DeploymentEvent[]>([])
  const [reconnectIn, setReconnectIn] = useState<number | null>(null)
  const wsRef = useRef<WebSocket | null>(null)
  const terminalRef = useRef(false)

  useEffect(() => {
    let cancelled = false
    let backoff = 1000
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null

    const connect = async () => {
      if (cancelled || terminalRef.current) return
      let token: string
      try {
        const r = await fetchWSToken()
        token = r.token
      } catch {
        return
      }
      if (cancelled) return
      const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      const url = `${proto}//${window.location.host}${urlPath}`
      const ws = new WebSocket(url, [`bandolier.ws.${token}`])
      wsRef.current = ws
      ws.onopen = () => {
        backoff = 1000
        setReconnectIn(null)
      }
      ws.onmessage = (e) => {
        try {
          const ev = JSON.parse(e.data) as DeploymentEvent
          setEvents((prev) => [...prev, ev])
          if (ev.type === 'deployment_complete') {
            terminalRef.current = true
          }
        } catch {
          // drop malformed event payloads silently — backend always emits valid JSON
        }
      }
      ws.onclose = (e) => {
        if (wsRef.current === ws) wsRef.current = null
        if (cancelled || terminalRef.current) return
        if (e.code === 1000) return
        const delay = Math.min(backoff, 30000)
        setReconnectIn(Math.ceil(delay / 1000))
        reconnectTimer = setTimeout(() => {
          backoff = Math.min(backoff * 2, 30000)
          connect()
        }, delay)
      }
    }

    connect()
    return () => {
      cancelled = true
      if (reconnectTimer) clearTimeout(reconnectTimer)
      wsRef.current?.close()
    }
  }, [urlPath])

  return { events, reconnectIn }
}

export function useDeploymentLogs(deploymentId: string): LogStreamReturn {
  return useReconnectingWS(`/ws/deployments/${deploymentId}/logs`)
}

export function useInstallLogs(installId: string): LogStreamReturn {
  return useReconnectingWS(`/ws/apps/installs/${installId}/logs`)
}
