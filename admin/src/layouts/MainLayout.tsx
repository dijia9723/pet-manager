import { useEffect, useLayoutEffect, useMemo, useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { ProLayout, type MenuDataItem } from '@ant-design/pro-components'
import { Dropdown, Watermark, theme } from 'antd'
import { IdcardOutlined, LogoutOutlined, UserOutlined } from '@ant-design/icons'
import { authApi } from '@/api/auth'
import { useAuthStore, homePathFor } from '@/store/auth'
import { usePreferencesStore } from '@/store/preferences'
import { useTabsStore } from '@/store/tabs'
import { usePermission } from '@/hooks/usePermission'
import { appConfig } from '@/config'
import { appRouteModules, findRouteByPath } from '@/router/helpers'
import { KeepAliveOutlet } from '@/components/KeepAliveOutlet'
import { BrandLogo } from '@/components/BrandLogo'
import { renderIcon, toMenuData } from './menu'
import { TabsBar } from './components/TabsBar'
import { HeaderActions } from './components/HeaderActions'
import { PreferencesDrawer } from './components/PreferencesDrawer'
import { LockPasswordModal } from './components/LockPasswordModal'
import { LockScreen } from './components/LockScreen'
import { lockOwner, useScreenLockStore } from '@/store/screenLock'

/** 主布局：菜单优先取后端下发的 /auth/menus，缺省回退本地路由表并按权限点过滤 */
export default function MainLayout() {
  const location = useLocation()
  const navigate = useNavigate()
  const [prefOpen, setPrefOpen] = useState(false)
  const [lockSetupOpen, setLockSetupOpen] = useState(false)
  const [lockOnSave, setLockOnSave] = useState(false)

  const user = useAuthStore((s) => s.user)
  const menus = useAuthStore((s) => s.menus)
  const accessToken = useAuthStore((s) => s.accessToken)
  const setUser = useAuthStore((s) => s.setUser)
  const setMenus = useAuthStore((s) => s.setMenus)
  const clearSession = useAuthStore((s) => s.clearSession)

  const prefs = usePreferencesStore()
  const { token } = theme.useToken()
  const { has, hasAny } = usePermission()

  // 刷新页面后拉取最新 profile 与菜单
  useEffect(() => {
    if (!accessToken) return
    let cancelled = false
    void (async () => {
      try {
        const [profile, tree] = await Promise.all([authApi.profile(), authApi.menus()])
        if (cancelled) return
        setUser(profile)
        setMenus(Array.isArray(tree) ? tree : [])
      } catch {
        // 401 由 axios 拦截器统一处理
      }
    })()
    return () => {
      cancelled = true
    }
  }, [accessToken, setUser, setMenus])

  // 本地兜底菜单：按权限点过滤 + hideInMenu
  const fallbackMenu = useMemo<MenuDataItem[]>(
    () =>
      appRouteModules
        .filter((route) => !route.hideInMenu)
        .filter(
          (route) =>
            (route.perm ? has(route.perm) : true) &&
            (route.anyOf?.length ? hasAny(route.anyOf) : true),
        )
        .map((route) => ({
          key: route.key,
          path: route.path,
          name: route.title,
          icon: renderIcon(route.icon),
        })),
    [has, hasAny],
  )

  const menuData = useMemo<MenuDataItem[]>(
    () => (menus.length > 0 ? toMenuData(menus) : fallbackMenu),
    [menus, fallbackMenu],
  )

  // 浏览器刷新恢复的页签，可能已无权访问，按当前权限裁剪
  useEffect(() => {
    const allowedPaths = appRouteModules
      .filter(
        (route) =>
          (route.perm ? has(route.perm) : true) &&
          (route.anyOf?.length ? hasAny(route.anyOf) : true),
      )
      .map((route) => route.path)
    useTabsStore.getState().pruneByPaths(allowedPaths)
  }, [has, hasAny])

  // 路由变化自动开签（在 layout 阶段同步入栈，避免页签闪烁）
  useLayoutEffect(() => {
    const route = findRouteByPath(location.pathname)
    if (!route) return
    useTabsStore.getState().openTab({
      path: location.pathname,
      title: route.title,
      icon: route.icon,
      affix: Boolean(route.affixTab) || location.pathname === homePathFor(user),
    })
  }, [location.pathname, user])

  const handleLogout = async () => {
    try {
      await authApi.logout()
    } catch {
      // 忽略登出接口异常，本地会话照常清理
    }
    useTabsStore.getState().resetTabs()
    clearSession()
    navigate('/login', { replace: true })
  }

  const isDark = prefs.themeMode === 'dark'
  const owner = lockOwner(user)
  const showLockSetup = () => {
    setLockOnSave(false)
    setLockSetupOpen(true)
    setPrefOpen(false)
  }
  const handleLock = () => {
    setPrefOpen(false)
    if (useScreenLockStore.getState().accounts[owner]) {
      useScreenLockStore.getState().lock(owner)
    } else {
      setLockOnSave(true)
      setLockSetupOpen(true)
    }
  }
  const renderBrand = (collapsed: boolean) => (
    <div className="app-sider-brand" title={appConfig.title}>
      <BrandLogo size={32} />
      {!collapsed && <span className="app-sider-brand-title">{appConfig.title}</span>}
    </div>
  )
  const watermarkLines = prefs.watermarkContent.trim()
    ? prefs.watermarkContent.trim().split(/\r?\n/).filter(Boolean)
    : [user?.nickname || user?.username || '管理后台', user?.tenant_name || '平台管理']

  return (
    <>
      <ProLayout
        title={appConfig.title}
        logo={false}
        menuHeaderRender={
          prefs.layout === 'mix'
            ? undefined
            : (_logo, _title, props) => renderBrand(Boolean(props?.collapsed))
        }
        headerTitleRender={() => renderBrand(false)}
        siderWidth={248}
        contentStyle={{ padding: 16 }}
        layout={prefs.layout}
        navTheme={isDark ? 'realDark' : 'light'}
        token={{
          sider: {
            colorBgMenuItemSelected: token.colorPrimaryBg,
            colorTextMenuSelected: token.colorPrimary,
            colorTextSubMenuSelected: token.colorPrimary,
            colorTextMenuActive: token.colorPrimary,
            colorTextMenuItemHover: token.colorPrimaryHover,
          },
          header: {
            colorBgMenuItemSelected: token.colorPrimaryBg,
            colorTextMenuSelected: token.colorPrimary,
            colorTextMenuActive: token.colorPrimary,
          },
        }}
        collapsed={prefs.collapsed}
        onCollapse={(collapsed) => prefs.setPref({ collapsed })}
        fixedHeader
        fixSiderbar
        location={{ pathname: location.pathname }}
        menuDataRender={() => menuData}
        menuItemRender={(item, dom) => <Link to={(item.path as string) || '/'}>{dom}</Link>}
        onMenuHeaderClick={() => navigate(homePathFor(user))}
        breadcrumbRender={(routes) => (prefs.breadcrumbEnable ? routes : [])}
        actionsRender={(props) => [
          <HeaderActions
            key="header-actions"
            onOpenPreferences={() => setPrefOpen(true)}
            onLock={handleLock}
            placement={props.layout === 'side' && !props.isMobile ? 'sider' : 'header'}
          />,
        ]}
        avatarProps={{
          icon: <UserOutlined />,
          size: 'small',
          title: user?.nickname || user?.username || '未登录',
          render: (_props, dom) => (
            <Dropdown
              menu={{
                items: [
                  {
                    key: 'profile',
                    icon: <IdcardOutlined />,
                    label: '个人中心',
                    onClick: () => navigate('/profile'),
                  },
                  { type: 'divider' },
                  {
                    key: 'logout',
                    icon: <LogoutOutlined />,
                    label: '退出登录',
                    onClick: handleLogout,
                  },
                ],
              }}
            >
              <span className="app-user-trigger" role="button" tabIndex={0} aria-label="账号菜单">
                {dom}
              </span>
            </Dropdown>
          ),
        }}
      >
        <TabsBar />
        <Watermark
          className="app-content-watermark"
          content={prefs.watermarkEnable ? watermarkLines : []}
          font={{
            color: `rgba(${isDark ? '255,255,255' : '42,48,58'},${prefs.watermarkOpacity / 100})`,
            fontSize: 14,
            fontWeight: 500,
          }}
          width={240}
          height={Math.max(64, watermarkLines.length * 22)}
          gap={[100, 88]}
          rotate={-22}
          zIndex={9}
        >
          <KeepAliveOutlet />
        </Watermark>
      </ProLayout>
      <PreferencesDrawer
        open={prefOpen}
        onClose={() => setPrefOpen(false)}
        onConfigureLock={showLockSetup}
        onLock={handleLock}
      />
      <LockPasswordModal
        open={lockSetupOpen}
        lockOnSave={lockOnSave}
        onClose={() => setLockSetupOpen(false)}
      />
      <LockScreen onLogout={handleLogout} />
    </>
  )
}
