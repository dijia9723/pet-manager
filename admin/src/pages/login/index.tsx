import { useEffect, useState } from 'react'
import { Navigate, useLocation, useNavigate } from 'react-router-dom'
import { authApi } from '@/api/auth'
import { useAuthStore, homePathFor } from '@/store/auth'
import { useTabsStore } from '@/store/tabs'
import { useMessage } from '@/hooks/useMessage'
import { appConfig } from '@/config'
import { PetScene } from './components/PetScene'
import { LoginPanel, type LoginValues } from './components/LoginPanel'
import './login.css'

export default function LoginPage() {
  const navigate = useNavigate()
  const location = useLocation()
  const accessToken = useAuthStore((s) => s.accessToken)
  const setSession = useAuthStore((s) => s.setSession)
  const setMenus = useAuthStore((s) => s.setMenus)
  const message = useMessage()
  const [submitting, setSubmitting] = useState(false)
  const [failTick, setFailTick] = useState(0)

  useEffect(() => {
    document.title = appConfig.title
  }, [])

  if (accessToken) {
    return <Navigate to="/" replace />
  }

  const from = (location.state as { from?: string } | null)?.from

  const handleSubmit = async (values: LoginValues) => {
    setSubmitting(true)
    try {
      const result = await authApi.login({
        username: values.username,
        password: values.password,
        // 平台管理员不填租户编码
        tenant_code: values.loginType === 'tenant' ? values.tenant_code?.trim() : undefined,
      })
      // 切换账号登录：清空上个会话遗留的页签
      useTabsStore.getState().resetTabs()
      setSession(result)
      try {
        const menus = await authApi.menus()
        setMenus(Array.isArray(menus) ? menus : [])
      } catch {
        // 菜单拉取失败不阻塞登录
      }
      message.success('登录成功')
      navigate(from && from !== '/login' ? from : homePathFor(result.user), { replace: true })
    } catch {
      // 错误提示由 axios 拦截器统一处理；卡片轻摇提示重试
      setFailTick((t) => t + 1)
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="login-page">
      <PetScene />
      <LoginPanel submitting={submitting} failTick={failTick} onFinish={handleSubmit} />
    </div>
  )
}
