import { Button, Dropdown, Tooltip } from 'antd'
import {
  BulbFilled,
  BulbOutlined,
  FullscreenExitOutlined,
  FullscreenOutlined,
  ReloadOutlined,
  SettingOutlined,
  LockOutlined,
  AppstoreOutlined,
} from '@ant-design/icons'
import { useLocation } from 'react-router-dom'
import { useFullscreen } from '@/hooks/useFullscreen'
import { usePreferencesStore } from '@/store/preferences'
import { useTabsStore } from '@/store/tabs'
import { NotificationBell } from './NotificationBell'

interface HeaderActionsProps {
  onOpenPreferences: () => void
  onLock: () => void
  placement?: 'header' | 'sider'
}

const iconButtonStyle = { fontSize: 16 } as const

/** 顶栏右侧工具：刷新当前页、全屏、明暗切换、偏好设置 */
export function HeaderActions({
  onOpenPreferences,
  onLock,
  placement = 'header',
}: HeaderActionsProps) {
  const { pathname } = useLocation()
  const { isFullscreen, toggleFullscreen } = useFullscreen()
  const themeMode = usePreferencesStore((s) => s.themeMode)
  const toggleTheme = usePreferencesStore((s) => s.toggleTheme)

  const handleRefresh = () => {
    useTabsStore.getState().refresh(pathname)
  }

  if (placement === 'sider') {
    return (
      <div className="app-header-actions">
        <NotificationBell />
        <Dropdown
          trigger={['click']}
          placement="topRight"
          menu={{
            className: 'app-quick-actions-menu',
            items: [
              {
                key: 'refresh',
                icon: <ReloadOutlined />,
                label: '刷新页面',
                onClick: handleRefresh,
              },
              {
                key: 'fullscreen',
                icon: isFullscreen ? <FullscreenExitOutlined /> : <FullscreenOutlined />,
                label: isFullscreen ? '退出全屏' : '全屏',
                onClick: toggleFullscreen,
              },
              {
                key: 'theme',
                icon: themeMode === 'dark' ? <BulbFilled /> : <BulbOutlined />,
                label: themeMode === 'dark' ? '切换亮色' : '切换暗色',
                onClick: toggleTheme,
              },
              { type: 'divider' },
              { key: 'lock', icon: <LockOutlined />, label: '锁屏', onClick: onLock },
              {
                key: 'preferences',
                icon: <SettingOutlined />,
                label: '偏好设置',
                onClick: onOpenPreferences,
              },
            ],
          }}
        >
          <Button
            type="text"
            className="app-quick-actions-trigger"
            aria-label="快捷操作"
            title="快捷操作"
            icon={<AppstoreOutlined style={iconButtonStyle} />}
          />
        </Dropdown>
      </div>
    )
  }

  return (
    <div className="app-header-actions">
      <NotificationBell />
      <Tooltip title="刷新页面">
        <Button
          type="text"
          aria-label="刷新页面"
          icon={<ReloadOutlined style={iconButtonStyle} />}
          onClick={handleRefresh}
        />
      </Tooltip>
      <Tooltip title={isFullscreen ? '退出全屏' : '全屏'}>
        <Button
          type="text"
          aria-label={isFullscreen ? '退出全屏' : '全屏'}
          icon={
            isFullscreen ? (
              <FullscreenExitOutlined style={iconButtonStyle} />
            ) : (
              <FullscreenOutlined style={iconButtonStyle} />
            )
          }
          onClick={toggleFullscreen}
        />
      </Tooltip>
      <Tooltip title={themeMode === 'dark' ? '切换亮色' : '切换暗色'}>
        <Button
          type="text"
          aria-label={themeMode === 'dark' ? '切换亮色' : '切换暗色'}
          icon={
            themeMode === 'dark' ? (
              <BulbFilled style={iconButtonStyle} />
            ) : (
              <BulbOutlined style={iconButtonStyle} />
            )
          }
          onClick={toggleTheme}
        />
      </Tooltip>
      <Tooltip title="锁屏">
        <Button
          type="text"
          aria-label="锁屏"
          icon={<LockOutlined style={iconButtonStyle} />}
          onClick={onLock}
        />
      </Tooltip>
      <Tooltip title="偏好设置">
        <Button
          type="text"
          aria-label="偏好设置"
          icon={<SettingOutlined style={iconButtonStyle} />}
          onClick={onOpenPreferences}
        />
      </Tooltip>
    </div>
  )
}
