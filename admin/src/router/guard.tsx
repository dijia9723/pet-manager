import { Navigate } from 'react-router-dom'
import { useAuthStore, homePathFor } from '@/store/auth'

/** 已登录时访问根路径，统一落地工作台 */
export function HomeRedirect() {
  const user = useAuthStore((s) => s.user)
  return <Navigate to={homePathFor(user)} replace />
}

export { AuthGuard } from '@/components/AuthGuard'
