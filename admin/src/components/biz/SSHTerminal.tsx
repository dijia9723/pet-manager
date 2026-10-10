import { useEffect, useRef, useState } from 'react'
import { Button, Drawer, Space, Tag, Typography } from 'antd'
import { Terminal } from '@xterm/xterm'
import { FitAddon } from '@xterm/addon-fit'
import '@xterm/xterm/css/xterm.css'
import { API_BASE_URL } from '@/api/client'
import type { ManagedServer } from '@/api/hosting'
import { getAccessToken } from '@/utils/token'

type Status = 'connecting' | 'connected' | 'disconnected' | 'error'
const statusLabels: Record<Status, string> = {
  connecting: '连接中',
  connected: '已连接',
  disconnected: '已断开',
  error: '连接失败',
}

function TerminalPane({ server }: { server: ManagedServer }) {
  const container = useRef<HTMLDivElement>(null)
  const socketRef = useRef<WebSocket | null>(null)
  const [status, setStatus] = useState<Status>('connecting')
  const [attempt, setAttempt] = useState(0)

  useEffect(() => {
    if (!container.current) return
    let disposed = false
    let ready = false
    let ended = false
    let socket: WebSocket | null = null
    let frame = 0
    let pendingOutput = 0
    const terminal = new Terminal({
      cursorBlink: true,
      fontSize: 14,
      fontFamily: 'Consolas, "Cascadia Mono", "Courier New", monospace',
      scrollback: 5000,
      disableStdin: true,
      theme: { background: '#10151e', foreground: '#e4e8ef', cursor: '#70d6b5' },
    })
    const fit = new FitAddon()
    terminal.loadAddon(fit)
    terminal.open(container.current)
    const dimensions = () => ({
      cols: Math.max(2, Math.min(500, terminal.cols)),
      rows: Math.max(1, Math.min(200, terminal.rows)),
    })
    const sendResize = () => {
      if (ready && socket?.readyState === WebSocket.OPEN) {
        socket.send(JSON.stringify({ type: 'resize', ...dimensions() }))
      }
    }
    const resize = () => {
      cancelAnimationFrame(frame)
      frame = requestAnimationFrame(() => {
        if (!disposed) fit.fit()
      })
    }
    const observer = new ResizeObserver(resize)
    observer.observe(container.current)
    const resizeListener = terminal.onResize(sendResize)
    const dataListener = terminal.onData((data) => {
      if (!ready || socket?.readyState !== WebSocket.OPEN) return
      // Unicode-aware chunks keep large pastes below the server's frame limit.
      for (const chunk of data.match(/[\s\S]{1,4096}/gu) ?? []) {
        socket.send(JSON.stringify({ type: 'input', data: chunk }))
      }
    })
    const binaryListener = terminal.onBinary((data) => {
      if (ready && socket?.readyState === WebSocket.OPEN) {
        socket.send(Uint8Array.from(data, (char) => char.charCodeAt(0)))
      }
    })

    void document.fonts.ready.then(() => {
      if (disposed) return
      fit.fit()
      const token = getAccessToken()
      if (!token) {
        terminal.writeln('登录已过期，请重新登录。')
        setStatus('error')
        return
      }
      const url = new URL(
        `${API_BASE_URL.replace(/\/$/, '')}/platform/servers/${server.id}/terminal`,
        window.location.href,
      )
      url.protocol = url.protocol === 'https:' ? 'wss:' : 'ws:'
      const size = dimensions()
      url.searchParams.set('cols', String(size.cols))
      url.searchParams.set('rows', String(size.rows))
      try {
        socket = new WebSocket(url, ['pet-manager-terminal.v1', `bearer.${token}`])
      } catch {
        terminal.writeln('无法创建终端连接，请检查登录状态及服务地址。')
        setStatus('error')
        return
      }
      socketRef.current = socket
      socket.binaryType = 'arraybuffer'
      socket.onmessage = (event: MessageEvent<string | ArrayBuffer>) => {
        if (disposed) return
        if (event.data instanceof ArrayBuffer) {
          const bytes = new Uint8Array(event.data)
          pendingOutput += bytes.byteLength
          if (pendingOutput > 4 << 20) {
            ended = true
            ready = false
            terminal.options.disableStdin = true
            setStatus('error')
            terminal.writeln('\r\n终端输出过快，连接已断开。')
            socket?.close(1000, 'output overflow')
            return
          }
          terminal.write(bytes, () => {
            pendingOutput -= bytes.byteLength
          })
          return
        }
        const message = JSON.parse(event.data) as {
          type: 'ready' | 'error' | 'exit'
          message?: string
          exit_code?: number
        }
        if (message.type === 'ready') {
          ready = true
          terminal.options.disableStdin = false
          setStatus('connected')
          fit.fit()
          sendResize()
          terminal.focus()
        } else if (message.type === 'error') {
          ended = true
          ready = false
          terminal.options.disableStdin = true
          setStatus('error')
          terminal.writeln(`\r\n${message.message || 'SSH 连接失败。'}`)
        } else if (message.type === 'exit') {
          ended = true
          ready = false
          terminal.options.disableStdin = true
          setStatus('disconnected')
          terminal.writeln(`\r\n会话已结束，退出码：${message.exit_code ?? 0}`)
        }
      }
      socket.onclose = (event) => {
        if (disposed) return
        ready = false
        terminal.options.disableStdin = true
        if (!ended) {
          setStatus(event.code === 1000 ? 'disconnected' : 'error')
          terminal.writeln(
            event.code === 1000
              ? '\r\n连接已断开。'
              : '\r\n终端连接已断开，请检查登录状态、网络或服务器连接后重试。',
          )
        }
      }
    })

    return () => {
      disposed = true
      observer.disconnect()
      cancelAnimationFrame(frame)
      dataListener.dispose()
      binaryListener.dispose()
      resizeListener.dispose()
      socket?.close(1000, 'terminal closed')
      socketRef.current = null
      terminal.dispose()
    }
  }, [server.id, attempt])

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column', gap: 12 }}>
      <Space wrap style={{ justifyContent: 'space-between' }}>
        <Space>
          <Tag color={status === 'connected' ? 'green' : status === 'error' ? 'red' : 'default'}>
            {statusLabels[status]}
          </Tag>
          <Typography.Text code>
            {server.username}@{server.host}:{server.port}
          </Typography.Text>
        </Space>
        <Space>
          <Button
            disabled={status === 'connecting'}
            onClick={() => {
              setStatus('connecting')
              setAttempt((value) => value + 1)
            }}
          >
            重新连接
          </Button>
          <Button
            disabled={status !== 'connected' && status !== 'connecting'}
            onClick={() => socketRef.current?.close(1000, 'user disconnected')}
          >
            断开连接
          </Button>
        </Space>
      </Space>
      <div
        ref={container}
        aria-label="SSH 交互终端"
        style={{
          flex: 1,
          minHeight: 240,
          minWidth: 0,
          padding: 12,
          background: '#10151e',
          borderRadius: 8,
          overflow: 'hidden',
        }}
      />
    </div>
  )
}

export default function SSHTerminal({
  server,
  onClose,
}: {
  server: ManagedServer | null
  onClose: () => void
}) {
  return (
    <Drawer
      title={server ? `终端 · ${server.name}` : 'SSH 终端'}
      width="94vw"
      open={!!server}
      onClose={onClose}
      destroyOnClose
      styles={{ body: { display: 'flex', flexDirection: 'column' } }}
    >
      {server && <TerminalPane key={server.id} server={server} />}
    </Drawer>
  )
}
