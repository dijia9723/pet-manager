import type { MenuProps } from 'antd'
import { Dropdown, Tabs } from 'antd'
import { useLocation, useNavigate } from 'react-router-dom'
import { useTabsStore, type TabItem } from '@/store/tabs'
import { usePreferencesStore } from '@/store/preferences'
import { renderIcon } from '@/layouts/menu'

type MenuAction = 'refresh' | 'close' | 'closeOthers' | 'closeLeft' | 'closeRight' | 'closeAll'

/**
 * 多标签页条（对标 Vben tabbar）：
 * - 路由变化由 MainLayout 自动开签，点签切换路由；
 * - 固定页签（首页 / 工作台）不可关闭；
 * - 右键菜单：刷新、关闭、关闭其他/左侧/右侧/全部；
 * - 开关与刷新后恢复由 preferences / tabs 两个持久化 store 负责。
 */
export function TabsBar() {
  const enable = usePreferencesStore((s) => s.tabbarEnable)
  const tabs = useTabsStore((s) => s.tabs)
  const location = useLocation()
  const navigate = useNavigate()

  if (!enable) return null

  /** 若当前页签被关闭，跳到剩余的最后一个页签 */
  const ensureActiveExists = (remaining: TabItem[]) => {
    if (!remaining.some((tab) => tab.path === location.pathname)) {
      navigate(remaining[remaining.length - 1]?.path ?? '/')
    }
  }

  const handleAction = (action: MenuAction, path: string) => {
    const store = useTabsStore.getState()

    switch (action) {
      case 'refresh':
        store.refresh(path)
        break
      case 'close':
        ensureActiveExists(store.closeTab(path))
        break
      case 'closeOthers':
        store.closeOthers(path)
        navigate(path)
        break
      case 'closeLeft':
        store.closeLeft(path)
        ensureActiveExists(useTabsStore.getState().tabs)
        break
      case 'closeRight':
        store.closeRight(path)
        ensureActiveExists(useTabsStore.getState().tabs)
        break
      case 'closeAll':
        ensureActiveExists(store.closeAll())
        break
    }
  }

  const buildMenu = (tab: TabItem): MenuProps['items'] => [
    { key: 'refresh', label: '刷新页面' },
    { type: 'divider' },
    { key: 'close', label: '关闭', disabled: tab.affix },
    { key: 'closeOthers', label: '关闭其他' },
    {
      key: 'closeLeft',
      label: '关闭左侧',
      disabled: tabs.findIndex((t) => t.path === tab.path) === 0,
    },
    {
      key: 'closeRight',
      label: '关闭右侧',
      disabled: tabs.findIndex((t) => t.path === tab.path) === tabs.length - 1,
    },
    { type: 'divider' },
    { key: 'closeAll', label: '关闭全部' },
  ]

  return (
    <div className="app-tabs-bar">
      <Tabs
        size="small"
        type="editable-card"
        hideAdd
        activeKey={location.pathname}
        onChange={(key) => navigate(key)}
        onEdit={(key, action) => {
          if (action === 'remove') {
            ensureActiveExists(useTabsStore.getState().closeTab(String(key)))
          }
        }}
        items={tabs.map((tab) => ({
          key: tab.path,
          closable: !tab.affix,
          label: (
            <Dropdown
              trigger={['contextMenu']}
              menu={{
                items: buildMenu(tab),
                onClick: ({ key }) => handleAction(key as MenuAction, tab.path),
              }}
            >
              <span className="app-tab-label">
                {renderIcon(tab.icon)}
                {tab.title}
              </span>
            </Dropdown>
          ),
        }))}
      />
    </div>
  )
}
