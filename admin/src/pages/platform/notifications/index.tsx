import { PageContainer } from '@/components/PageContainer'
import { NotificationManager } from '@/components/biz/NotificationManager'

export default function NotificationsPage() {
  return (
    <PageContainer>
      <NotificationManager scope="platform" />
    </PageContainer>
  )
}
