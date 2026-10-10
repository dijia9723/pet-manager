import { Badge, Button, Tooltip } from 'antd'
import { BellOutlined } from '@ant-design/icons'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { useEffect } from 'react'
import { messageApi } from '@/api/notification'
import { useAuthStore } from '@/store/auth'
import { unreadEvent } from '@/utils/notifications'

export function NotificationBell() {
  const user = useAuthStore((s) => s.user)
  const navigate = useNavigate()
  const client = useQueryClient()
  const { data = 0 } = useQuery({
    queryKey: ['message-unread', user?.user_type, user?.tenant_id, user?.id],
    queryFn: () => messageApi(user!.user_type).unread(),
    enabled: !!user,
    refetchInterval: 30000,
    retry: false,
  })
  useEffect(() => {
    const refresh = () => {
      void client.invalidateQueries({ queryKey: ['message-unread'] })
    }
    window.addEventListener(unreadEvent, refresh)
    return () => window.removeEventListener(unreadEvent, refresh)
  }, [client])
  return (
    <Tooltip title={`消息中心${data ? `（${data} 条未读）` : ''}`}>
      <Button
        type="text"
        aria-label={`消息中心，${data} 条未读`}
        onClick={() => navigate('/messages')}
        icon={
          <Badge count={data} size="small" offset={[3, -2]}>
            <BellOutlined style={{ fontSize: 16 }} />
          </Badge>
        }
      />
    </Tooltip>
  )
}
