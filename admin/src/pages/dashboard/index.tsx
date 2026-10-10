import { PageContainer } from '@/components/PageContainer'
import { Card, Col, Row, Tag } from 'antd'
import {
  IdcardOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  TeamOutlined,
  UserOutlined,
} from '@ant-design/icons'
import { BrandLogo } from '@/components/BrandLogo'
import { useAuthStore } from '@/store/auth'

function greeting(): string {
  const h = new Date().getHours()
  if (h < 6) return '夜深了'
  if (h < 9) return '早上好'
  if (h < 12) return '上午好'
  if (h < 14) return '中午好'
  if (h < 18) return '下午好'
  return '晚上好'
}

interface StatItem {
  label: string
  value: string
  icon: React.ReactNode
  iconBg: string
  iconColor: string
}

export default function DashboardPage() {
  const user = useAuthStore((s) => s.user)
  const isPlatform = user?.user_type === 'platform'

  const roles = user?.roles ?? []
  const permissions = user?.permissions ?? []

  const stats: StatItem[] = [
    {
      label: '登录账号',
      value: user?.username ?? '-',
      icon: <UserOutlined />,
      iconBg: 'var(--ant-color-primary-bg)',
      iconColor: 'var(--ant-color-primary)',
    },
    {
      label: '昵称',
      value: user?.nickname || '-',
      icon: <IdcardOutlined />,
      iconBg: 'var(--ant-color-success-bg)',
      iconColor: 'var(--ant-color-success)',
    },
    {
      label: '身份',
      value: isPlatform ? '平台管理员' : '租户账号',
      icon: <SafetyCertificateOutlined />,
      iconBg: 'var(--ant-color-warning-bg)',
      iconColor: 'var(--ant-color-warning)',
    },
    {
      label: isPlatform ? '平台' : '所属租户',
      value: isPlatform
        ? '运营方'
        : user?.tenant_name
          ? `${user.tenant_name}（${user.tenant_code}）`
          : '-',
      icon: <ShopOutlined />,
      iconBg: 'var(--ant-color-info-bg)',
      iconColor: 'var(--ant-color-info)',
    },
  ]

  return (
    <PageContainer title="工作台">
      <Card className="app-welcome" bordered={false}>
        <div className="welcome-paw">
          <BrandLogo size={150} plain />
        </div>
        <div className="welcome-hello">
          {greeting()}，{user?.nickname || user?.username || '欢迎回来'}
        </div>
        <div className="welcome-sub">
          {isPlatform
            ? '你正以平台管理员身份运营全站，可管理租户、审计日志与权限字典。'
            : `欢迎回到 ${user?.tenant_name ?? '工作台'}，今天也要照顾好每一只毛孩子。`}
        </div>
      </Card>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        {stats.map((item) => (
          <Col key={item.label} xs={24} sm={12} xl={6}>
            <Card className="app-stat-card" bordered={false}>
              <span
                className="app-stat-icon"
                style={{ background: item.iconBg, color: item.iconColor }}
              >
                {item.icon}
              </span>
              <div>
                <div className="app-stat-label">{item.label}</div>
                <div className="app-stat-value" title={item.value}>
                  {item.value}
                </div>
              </div>
            </Card>
          </Col>
        ))}
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={24} lg={10}>
          <Card
            bordered={false}
            style={{ borderRadius: 12, height: '100%' }}
            title={
              <span>
                <TeamOutlined style={{ marginRight: 8 }} />
                我的角色
              </span>
            }
          >
            {roles.length > 0 ? (
              roles.map((role) => (
                <Tag key={role} color="blue" style={{ marginBottom: 8 }}>
                  {role}
                </Tag>
              ))
            ) : (
              <span style={{ color: 'var(--ant-color-text-secondary)' }}>暂无角色</span>
            )}
            <div style={{ marginTop: 12, color: 'var(--ant-color-text-secondary)', fontSize: 13 }}>
              角色由管理员分配，决定你可见的菜单与可执行的操作。
            </div>
          </Card>
        </Col>
        <Col xs={24} lg={14}>
          <Card
            bordered={false}
            style={{ borderRadius: 12, height: '100%' }}
            title={
              <span>
                <SafetyCertificateOutlined style={{ marginRight: 8 }} />
                权限点（{permissions.length}）
              </span>
            }
          >
            {permissions.length > 0 ? (
              permissions.map((perm) => (
                <Tag key={perm} style={{ marginBottom: 8 }}>
                  {perm}
                </Tag>
              ))
            ) : (
              <span style={{ color: 'var(--ant-color-text-secondary)' }}>暂无权限点</span>
            )}
          </Card>
        </Col>
      </Row>
    </PageContainer>
  )
}
