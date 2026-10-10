import {
  Button,
  ColorPicker,
  Divider,
  Drawer,
  Input,
  Segmented,
  Slider,
  Space,
  Switch,
  Typography,
} from 'antd'
import { CheckOutlined, LockOutlined } from '@ant-design/icons'
import type { LayoutMode, ThemeMode, ThemePreset } from '@/store/preferences'
import { usePreferencesStore } from '@/store/preferences'
import { CUTE_PRIMARY } from '@/config/theme'
import { useAuthStore } from '@/store/auth'
import { lockOwner, useScreenLockStore } from '@/store/screenLock'

interface PreferencesDrawerProps {
  open: boolean
  onClose: () => void
  onConfigureLock: () => void
  onLock: () => void
}

const PRESET_COLORS = ['#1677ff', '#13c2c2', '#52c41a', '#722ed1', '#fa8c16', '#f5222d']

const PRESET_OPTIONS: Array<{
  value: ThemePreset
  label: string
  desc: string
  swatches: string[]
}> = [
  {
    value: 'classic',
    label: '经典商务',
    desc: '冷静蓝灰 · 专业',
    swatches: ['#1677ff', '#f0f5ff', '#ffffff'],
  },
  {
    value: 'cute',
    label: '萌宠奶油',
    desc: '蜜桃暖色 · 可爱',
    swatches: ['#f2814f', '#fde4d2', '#fdf3e9'],
  },
]

const BLOCK_TITLE: React.CSSProperties = { fontWeight: 600, marginBottom: 12 }

/** 偏好设置抽屉（对标 Vben preferences，覆盖布局 / 主题 / 页签 / 面包屑 / 水印） */
export function PreferencesDrawer({
  open,
  onClose,
  onConfigureLock,
  onLock,
}: PreferencesDrawerProps) {
  const prefs = usePreferencesStore()
  const setPref = usePreferencesStore((s) => s.setPref)
  const reset = usePreferencesStore((s) => s.reset)
  const owner = lockOwner(useAuthStore((s) => s.user))
  const lockConfigured = useScreenLockStore((s) => Boolean(s.accounts[owner]))

  return (
    <Drawer
      title="偏好设置"
      width={360}
      open={open}
      onClose={onClose}
      destroyOnHidden
      footer={
        <Button block onClick={reset}>
          恢复默认
        </Button>
      }
    >
      <Space direction="vertical" size={20} style={{ width: '100%' }}>
        <div>
          <div style={BLOCK_TITLE}>主题风格</div>
          <div className="app-preset-picker">
            {PRESET_OPTIONS.map((preset) => {
              const active = prefs.themePreset === preset.value
              return (
                <button
                  key={preset.value}
                  type="button"
                  className={`app-preset-card${active ? ' is-active' : ''}`}
                  aria-pressed={active}
                  onClick={() =>
                    setPref({
                      themePreset: preset.value,
                      colorPrimary: preset.value === 'cute' ? CUTE_PRIMARY : '#1677ff',
                    })
                  }
                >
                  <span className="app-preset-swatches">
                    {preset.swatches.map((color) => (
                      <i key={color} style={{ backgroundColor: color }} />
                    ))}
                  </span>
                  <span className="app-preset-meta">
                    <b>{preset.label}</b>
                    <small>{preset.desc}</small>
                  </span>
                  <span className="app-preset-check">{active && <CheckOutlined />}</span>
                </button>
              )
            })}
          </div>
        </div>

        <div>
          <div style={BLOCK_TITLE}>明暗模式</div>
          <Segmented<ThemeMode>
            block
            value={prefs.themeMode}
            onChange={(value) => setPref({ themeMode: value })}
            options={[
              { label: '亮色', value: 'light' },
              { label: '暗色', value: 'dark' },
            ]}
          />
        </div>

        <div>
          <div style={BLOCK_TITLE}>主题色</div>
          <Space wrap size={12}>
            {[...PRESET_COLORS, CUTE_PRIMARY].map((color) => (
              <button
                key={color}
                type="button"
                aria-label={`主题色 ${color}`}
                aria-pressed={prefs.colorPrimary.toLowerCase() === color.toLowerCase()}
                onClick={() => setPref({ colorPrimary: color })}
                style={{
                  width: 24,
                  height: 24,
                  backgroundColor: color,
                  borderRadius: '50%',
                  border:
                    prefs.colorPrimary.toLowerCase() === color.toLowerCase()
                      ? '2px solid var(--ant-color-bg-container)'
                      : '2px solid transparent',
                  outline: `3px solid ${color}`,
                  outlineOffset: 1,
                  boxShadow:
                    prefs.colorPrimary.toLowerCase() === color.toLowerCase()
                      ? '0 0 0 1px var(--ant-color-border)'
                      : 'none',
                  cursor: 'pointer',
                  padding: 0,
                }}
              >
                {prefs.colorPrimary.toLowerCase() === color.toLowerCase() && (
                  <CheckOutlined style={{ color: '#fff', fontSize: 12 }} />
                )}
              </button>
            ))}
            <ColorPicker
              value={prefs.colorPrimary}
              disabledAlpha
              onChangeComplete={(color) => setPref({ colorPrimary: color.toHexString() })}
            />
          </Space>
          <Typography.Text
            type="secondary"
            style={{ display: 'block', marginTop: 12, fontSize: 12 }}
          >
            当前主色：{prefs.colorPrimary} · 切换后立即生效
          </Typography.Text>
        </div>

        <Divider style={{ margin: 0 }} />

        <div>
          <div style={BLOCK_TITLE}>布局模式</div>
          <Segmented<LayoutMode>
            block
            value={prefs.layout}
            onChange={(value) => setPref({ layout: value })}
            options={[
              { label: '侧边', value: 'side' },
              { label: '混合', value: 'mix' },
              { label: '顶部', value: 'top' },
            ]}
          />
        </div>

        <div>
          <div style={BLOCK_TITLE}>界面</div>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <div style={ROW_STYLE}>
              <Typography.Text>多标签页</Typography.Text>
              <Switch
                checked={prefs.tabbarEnable}
                onChange={(checked) => setPref({ tabbarEnable: checked })}
              />
            </div>
            <div style={ROW_STYLE}>
              <Typography.Text>页面缓存（切走保留状态）</Typography.Text>
              <Switch
                checked={prefs.keepAliveEnable}
                onChange={(checked) => setPref({ keepAliveEnable: checked })}
              />
            </div>
            <div style={ROW_STYLE}>
              <Typography.Text>面包屑</Typography.Text>
              <Switch
                checked={prefs.breadcrumbEnable}
                onChange={(checked) => setPref({ breadcrumbEnable: checked })}
              />
            </div>
          </Space>
        </div>

        <Divider style={{ margin: 0 }} />
        <div>
          <div style={{ ...ROW_STYLE, marginBottom: 12 }}>
            <span style={{ fontWeight: 600 }}>页面水印</span>
            <Switch
              aria-label="页面水印"
              checked={prefs.watermarkEnable}
              onChange={(checked) => setPref({ watermarkEnable: checked })}
            />
          </div>
          <Input.TextArea
            aria-label="水印内容"
            value={prefs.watermarkContent}
            onChange={(event) => setPref({ watermarkContent: event.target.value })}
            placeholder="留空使用当前账号与所属租户，可换行"
            maxLength={60}
            autoSize={{ minRows: 2, maxRows: 3 }}
            disabled={!prefs.watermarkEnable}
          />
          <div style={{ ...ROW_STYLE, marginTop: 12 }}>
            <Typography.Text>清晰度</Typography.Text>
            <Typography.Text type="secondary">{prefs.watermarkOpacity}%</Typography.Text>
          </div>
          <Slider
            aria-label="水印清晰度"
            min={8}
            max={30}
            value={prefs.watermarkOpacity}
            disabled={!prefs.watermarkEnable}
            onChange={(value) => setPref({ watermarkOpacity: value })}
          />
        </div>
        <Divider style={{ margin: 0 }} />
        <div>
          <div style={BLOCK_TITLE}>锁屏保护</div>
          <Typography.Paragraph type="secondary" style={{ fontSize: 12 }}>
            {lockConfigured
              ? '已设置密码，离开工位时可随时锁屏。'
              : '设置自定义密码，防止离开工位后被误操作。'}
          </Typography.Paragraph>
          <Space>
            <Button onClick={onConfigureLock}>{lockConfigured ? '修改密码' : '设置密码'}</Button>
            <Button icon={<LockOutlined />} disabled={!lockConfigured} onClick={onLock}>
              立即锁屏
            </Button>
          </Space>
        </div>
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          偏好自动保存在此浏览器，下次打开继续使用。
        </Typography.Text>
      </Space>
    </Drawer>
  )
}

const ROW_STYLE: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
}
