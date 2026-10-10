import { Empty } from 'antd'
import { BrandLogo } from './BrandLogo'

/**
 * 全站统一空状态：浅灰爪印插画 + 描述文案。
 * 颜色用 token，亮暗模式自动适配。
 */
export function AppEmpty({ description = '暂无数据' }: { description?: string }) {
  return (
    <Empty
      image={
        <span className="app-empty-paw">
          <BrandLogo size={72} plain />
        </span>
      }
      description={<span style={{ color: 'var(--ant-color-text-tertiary)' }}>{description}</span>}
    />
  )
}
