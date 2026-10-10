import { useLayoutEffect, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { Alert, Avatar, Button, Form, Input, Typography } from 'antd'
import { LockOutlined, UnlockOutlined, UserOutlined } from '@ant-design/icons'
import { useAuthStore } from '@/store/auth'
import { lockOwner, useScreenLockStore } from '@/store/screenLock'
import { BrandLogo } from '@/components/BrandLogo'
import { appConfig } from '@/config'

export function LockScreen({ onLogout }: { onLogout: () => Promise<void> }) {
  const user = useAuthStore((s) => s.user)
  const owner = lockOwner(user)
  const locked = useScreenLockStore((s) => Boolean(s.accounts[owner]?.locked))
  const rootRef = useRef<HTMLElement>(null)
  const [form] = Form.useForm()
  const [error, setError] = useState('')
  const [pending, setPending] = useState(false)

  useLayoutEffect(() => {
    if (!locked) return
    const screen = rootRef.current!
    const previous = new Map<HTMLElement, boolean>()
    const disableBackground = () => {
      for (const child of document.body.children) {
        if (!(child instanceof HTMLElement) || child === screen || previous.has(child)) continue
        previous.set(child, child.inert)
        child.inert = true
      }
    }
    disableBackground()
    // 同时阻止已有和新打开的业务弹窗、抽屉及其他 portal 接收鼠标与键盘输入。
    const observer = new MutationObserver(disableBackground)
    observer.observe(document.body, { childList: true })
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    const focusInput = () => screen.querySelector('input')?.focus()
    focusInput()
    const keepFocus = (event: FocusEvent) => {
      if (!screen.contains(event.target as Node)) focusInput()
    }
    const trapKeyboard = (event: KeyboardEvent) => {
      if (event.key === 'Escape') event.preventDefault()
      if (event.key !== 'Tab') return
      const controls = Array.from(
        screen.querySelectorAll<HTMLElement>(
          'input:not(:disabled), button:not(:disabled), [tabindex="0"]',
        ),
      )
      const first = controls[0]
      const last = controls[controls.length - 1]
      if (
        event.shiftKey &&
        (document.activeElement === first || !screen.contains(document.activeElement))
      ) {
        event.preventDefault()
        last?.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first?.focus()
      }
    }
    document.addEventListener('focusin', keepFocus)
    document.addEventListener('keydown', trapKeyboard, true)
    return () => {
      observer.disconnect()
      previous.forEach((value, element) => {
        element.inert = value
      })
      document.body.style.overflow = overflow
      document.removeEventListener('focusin', keepFocus)
      document.removeEventListener('keydown', trapKeyboard, true)
    }
  }, [locked])

  if (!locked) return null

  const unlock = async ({ password }: { password: string }) => {
    setPending(true)
    setError('')
    try {
      const success = await useScreenLockStore.getState().unlock(owner, password)
      form.resetFields()
      if (!success) setError('锁屏密码不正确，请重试')
    } catch (error) {
      setError(error instanceof Error ? error.message : '解锁失败，请重试')
    } finally {
      setPending(false)
    }
  }

  return createPortal(
    <section
      ref={rootRef}
      className="app-lock-screen"
      role="dialog"
      aria-modal="true"
      aria-labelledby="screen-lock-title"
    >
      <div className="app-lock-brand">
        <BrandLogo size={32} />
        <span>{appConfig.title}</span>
      </div>
      <div className="app-lock-card">
        <span className="app-lock-badge">
          <LockOutlined />
        </span>
        <Typography.Title id="screen-lock-title" level={3}>
          屏幕已锁定
        </Typography.Title>
        <Typography.Paragraph type="secondary">
          后台操作已暂停，输入锁屏密码继续工作
        </Typography.Paragraph>
        <div className="app-lock-user">
          <Avatar icon={<UserOutlined />} />
          <div>
            <strong>{user?.nickname || user?.username}</strong>
            <small>{user?.tenant_name || '平台管理'}</small>
          </div>
        </div>
        {error && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        <Form form={form} onFinish={unlock} layout="vertical" disabled={pending}>
          <Form.Item
            name="password"
            label="锁屏密码"
            rules={[{ required: true, message: '请输入锁屏密码' }]}
          >
            <Input.Password
              size="large"
              autoComplete="current-password"
              placeholder="请输入锁屏密码"
              maxLength={64}
              prefix={<LockOutlined />}
            />
          </Form.Item>
          <Button
            block
            size="large"
            type="primary"
            htmlType="submit"
            loading={pending}
            icon={<UnlockOutlined />}
          >
            解锁并继续
          </Button>
        </Form>
        <Button
          block
          type="link"
          disabled={pending}
          style={{ marginTop: 16 }}
          onClick={async () => {
            setPending(true)
            try {
              await onLogout()
            } finally {
              setPending(false)
            }
          }}
        >
          退出登录，重新验证身份
        </Button>
      </div>
    </section>,
    document.body,
  )
}
