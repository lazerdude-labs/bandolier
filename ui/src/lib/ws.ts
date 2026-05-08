import { useEffect, useRef, useState } from 'react'
import { fetchWSToken } from './api'

export interface DeploymentEvent {
  type: 'step_start' | 'step_end' | 'log' | 'ansible_event' | 'deployment_complete'
  step?: string
  stream?: string
  text?: string
  status?: string
  exit_code?: number
  data?: any
  ts: string
}

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
        } catch {}
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
