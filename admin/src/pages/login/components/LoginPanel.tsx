import { useCallback, useEffect, useRef } from 'react'
import {
  ApartmentOutlined,
  CloudServerOutlined,
  LockOutlined,
  ShopOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { LoginForm, ProFormDependency, ProFormRadio, ProFormText } from '@ant-design/pro-components'
import { BrandLogo } from '@/components/BrandLogo'
import type { UserType } from '@/types'
import { HeartMark, PawMark, SparkleMark } from './icons'

export interface LoginValues {
  username: string
  password: string
  tenant_code?: string
  loginType: UserType
}

interface LoginPanelProps {
  submitting: boolean
  /** 登录失败序号：变化时卡片轻摇，提示用户重新输入 */
  failTick: number
  onFinish: (values: LoginValues) => Promise<void>
}

/**
 * 右侧欢迎卡片：保留 ProComponents LoginForm 的全部校验/提交流程，
 * 视觉风格由 login.css 限定在 .login-card 作用域内覆写。
 * 卡片带轻微的鼠标 3D 倾斜反馈，登录失败时整体轻摇。
 */
export function LoginPanel({ submitting, failTick, onFinish }: LoginPanelProps) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const cardRef = useRef<HTMLDivElement>(null)
  const tiltEnabled = useRef(
    typeof window !== 'undefined' && !window.matchMedia('(prefers-reduced-motion: reduce)').matches,
  )

  /* 登录失败：卡片轻摇（重播关键帧） */
  useEffect(() => {
    if (!failTick) return
    const el = wrapRef.current
    if (!el) return
    el.classList.remove('is-shake')
    void el.offsetWidth
    el.classList.add('is-shake')
  }, [failTick])

  const handleShakeEnd = useCallback(() => {
    wrapRef.current?.classList.remove('is-shake')
  }, [])

  const handleMouseMove = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const card = cardRef.current
    if (!card || !tiltEnabled.current) return
    const rect = card.getBoundingClientRect()
    const rx = (((e.clientY - rect.top) / rect.height) * 2 - 1) * -4
    const ry = (((e.clientX - rect.left) / rect.width) * 2 - 1) * 4
    card.style.setProperty('--rx', `${rx.toFixed(2)}deg`)
    card.style.setProperty('--ry', `${ry.toFixed(2)}deg`)
  }, [])

  const handleMouseLeave = useCallback(() => {
    cardRef.current?.style.setProperty('--rx', '0deg')
    cardRef.current?.style.setProperty('--ry', '0deg')
  }, [])

  return (
    <div className="login-panel">
      {/* 右侧补景：柔光色块 + 巨型爪印水印 + 漂浮小元素，填补宽屏下的空白带 */}
      <div className="login-panel-deco" aria-hidden>
        <span className="lp-blob lp-blob-1" />
        <span className="lp-blob lp-blob-2" />
        <PawMark className="lp-watermark" style={{ width: 300 }} />
        <PawMark className="lp-floater lp-floater-1" style={{ width: 44 }} />
        <HeartMark className="lp-floater lp-floater-2" style={{ width: 24 }} />
        <SparkleMark className="lp-twinkle" style={{ width: 16 }} />
      </div>

      <div ref={wrapRef} className="login-card-wrap" onAnimationEnd={handleShakeEnd}>
        <div ref={cardRef} className="login-card" onMouseMove={handleMouseMove} onMouseLeave={handleMouseLeave}>
          <div className="login-card-header">
            <BrandLogo size={52} plain className="login-card-paw" />
            <h1 className="login-card-title">欢迎回来</h1>
            <p className="login-card-subtitle">登录你的宠物日常乐园</p>
          </div>

          <LoginForm<LoginValues>
            initialValues={{ loginType: 'platform' }}
            submitter={{
              searchConfig: { submitText: '登 录' },
              submitButtonProps: { loading: submitting, size: 'large' },
            }}
            onFinish={onFinish}
          >
            <ProFormRadio.Group
              name="loginType"
              label="登录身份"
              fieldProps={{
                optionType: 'button',
                buttonStyle: 'solid',
                style: { width: '100%' },
              }}
              options={[
                {
                  label: (
                    <span className="lp-radio-label">
                      <CloudServerOutlined />
                      平台管理员
                    </span>
                  ),
                  value: 'platform',
                },
                {
                  label: (
                    <span className="lp-radio-label">
                      <ShopOutlined />
                      租户账号
                    </span>
                  ),
                  value: 'tenant',
                },
              ]}
            />
            <ProFormText
              name="username"
              label="账号"
              placeholder="请输入账号"
              fieldProps={{ prefix: <UserOutlined />, autoComplete: 'username' }}
              rules={[{ required: true, message: '请输入账号' }]}
            />
            <ProFormText.Password
              name="password"
              label="密码"
              placeholder="请输入密码"
              fieldProps={{ prefix: <LockOutlined />, autoComplete: 'current-password' }}
              rules={[{ required: true, message: '请输入密码' }]}
            />
            <ProFormDependency name={['loginType']}>
              {({ loginType }) =>
                loginType === 'tenant' ? (
                  <ProFormText
                    name="tenant_code"
                    label="租户编码"
                    placeholder="请输入租户编码"
                    fieldProps={{ prefix: <ApartmentOutlined />, autoComplete: 'organization' }}
                    formItemProps={{ className: 'lp-field-enter' }}
                    rules={[{ required: true, message: '请输入租户编码' }]} />
                ) : null
              }
            </ProFormDependency>
          </LoginForm>
        </div>
      </div>
      <p className="login-footer">宠物商户 SaaS 管理后台 · 让每一只毛孩子都被温柔以待</p>
    </div>
  )
}
