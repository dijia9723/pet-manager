import {
  PageContainer as ProPageContainer,
  type PageContainerProps,
} from '@ant-design/pro-components'
import { usePreferencesStore } from '@/store/preferences'
import { Link } from 'react-router-dom'

/** 所有业务页共享界面偏好，避免 PageContainer 自己再次生成面包屑。 */
export function PageContainer(props: PageContainerProps) {
  const enabled = usePreferencesStore((s) => s.breadcrumbEnable)
  return (
    <ProPageContainer
      {...props}
      breadcrumb={
        enabled
          ? (props.breadcrumb ?? {
              items: [
                { title: <Link to="/dashboard">首页</Link> },
                ...(props.title ? [{ title: props.title }] : []),
              ],
            })
          : { items: [] }
      }
      breadcrumbRender={enabled ? props.breadcrumbRender : false}
    />
  )
}
