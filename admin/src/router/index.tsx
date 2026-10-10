import { Fragment, Suspense, lazy } from 'react'
import { createBrowserRouter } from 'react-router-dom'
import { AuthGuard } from '@/components/AuthGuard'
import { ChunkErrorBoundary } from '@/components/ChunkErrorBoundary'
import { PageLoading } from '@/components/PageLoading'
import { RouteTracker } from '@/components/RouteTracker'
import MainLayout from '@/layouts/MainLayout'
import LoginPage from '@/pages/login'
import { appRouteModules } from './routes'
import { HomeRedirect } from './guard'

// 框架页体积小且为兜底链路，保持静态加载
const ForbiddenPage = lazy(() => import('@/pages/errors/403'))
const NotFoundPage = lazy(() => import('@/pages/errors/404'))

/**
 * 路由表：静态框架路由 + 业务路由（懒加载 + AuthGuard 登录/权限点双重校验）。
 * 菜单另由后端 /auth/menus 下发（已按权限过滤），路由模块表用于落地与本地兜底。
 */
export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
  {
    path: '/',
    element: (
      <AuthGuard>
        <MainLayout />
      </AuthGuard>
    ),
    children: [
      { index: true, element: <HomeRedirect /> },
      ...appRouteModules.map((route) => {
        const Page = route.component
        return {
          path: route.path.replace(/^\//, ''),
          element: (
            <AuthGuard perm={route.perm} anyOf={route.anyOf}>
              <ChunkErrorBoundary>
                <Suspense fallback={<PageLoading />}>
                  <Fragment>
                    <RouteTracker path={route.path} title={route.title} />
                    <Page />
                  </Fragment>
                </Suspense>
              </ChunkErrorBoundary>
            </AuthGuard>
          ),
        }
      }),
      {
        path: '403',
        element: (
          <Suspense fallback={<PageLoading />}>
            <ForbiddenPage />
          </Suspense>
        ),
      },
      {
        path: '404',
        element: (
          <Suspense fallback={<PageLoading />}>
            <NotFoundPage />
          </Suspense>
        ),
      },
      {
        path: '*',
        element: (
          <Suspense fallback={<PageLoading />}>
            <NotFoundPage />
          </Suspense>
        ),
      },
    ],
  },
])
