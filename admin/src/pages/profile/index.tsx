import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { PageContainer } from '@/components/PageContainer'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/store/auth'
import { useTabsStore } from '@/store/tabs'
import { useMessage } from '@/hooks/useMessage'
import {
  Avatar,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Row,
  Tabs,
  Tag,
} from 'antd'
import {
  IdcardOutlined,
  LockOutlined,
  SafetyCertificateOutlined,
  ShopOutlined,
  UserOutlined,
} from '@ant-design/icons'
import type { ChangePasswordPayload } from '@/types'

interface ProfileFormValues {
  nickname: string
}

interface PasswordFormValues extends ChangePasswordPayload {
  confirm_password: string
}

/** 个人中心：左侧身份卡 + 右侧「基本资料 / 修改密码」两个页签，平台与租户账号通用 */
export default function ProfilePage() {
  const navigate = useNavigate()
  const message = useMessage()
  const user = useAuthStore((s) => s.user)
  const setUser = useAuthStore((s) => s.setUser)
  const clearSession = useAuthStore((s) => s.clearSession)

  const [profileForm] = Form.useForm<ProfileFormValues>()
  const [passwordForm] = Form.useForm<PasswordFormValues>()
  const [savingProfile, setSavingProfile] = useState(false)
  const [savingPassword, setSavingPassword] = useState(false)

  const isPlatform = user?.user_type === 'platform'
  const roles = user?.roles ?? []

  const handleSaveProfile = async (values: ProfileFormValues) => {
    setSavingProfile(true)
    try {
      const updated = await authApi.updateProfile({ nickname: values.nickname.trim() })
      if (updated) setUser(updated)
      message.success('资料已更新')
    } finally {
      setSavingProfile(false)
    }
  }

  const handleChangePassword = async (values: PasswordFormValues) => {
    setSavingPassword(true)
    try {
      await authApi.changePassword({
        old_password: values.old_password,
        new_password: values.new_password,
      })
      // 后端已踢掉全部会话（含当前会话），本地清理后引导重新登录
      useTabsStore.getState().resetTabs()
      clearSession()
      message.success('密码修改成功，请使用新密码重新登录')
      navigate('/login', { replace: true })
    } finally {
      setSavingPassword(false)
    }
  }

  return (
    <PageContainer title="个人中心">
      <Row gutter={[16, 16]}>
        <Col xs={24} lg={8} xxl={6}>
          <Card className="app-profile-card" bordered={false}>
            <div className="app-profile-head">
              <Avatar size={64} icon={<UserOutlined />} className="app-profile-avatar" />
              <div className="app-profile-head-meta">
                <div className="app-profile-name" title={user?.nickname || user?.username}>
                  {user?.nickname || user?.username || '-'}
                </div>
                <div className="app-profile-sub">@{user?.username ?? '-'}</div>
                <div className="app-profile-tags">
                  <Tag color={isPlatform ? 'gold' : 'blue'} className="app-profile-tag">
                    {isPlatform ? '平台管理员' : '租户账号'}
                  </Tag>
                  {roles.slice(0, 2).map((role) => (
                    <Tag key={role} className="app-profile-tag">
                      {role}
                    </Tag>
                  ))}
                </div>
              </div>
            </div>
            <Descriptions column={1} size="small" className="app-profile-desc">
              <Descriptions.Item label="登录账号">{user?.username ?? '-'}</Descriptions.Item>
              <Descriptions.Item
                label={
                  <span>
                    <SafetyCertificateOutlined style={{ marginRight: 6 }} />
                    身份
                  </span>
                }
              >
                {isPlatform ? '平台管理员' : '租户子账号'}
              </Descriptions.Item>
              <Descriptions.Item
                label={
                  <span>
                    <ShopOutlined style={{ marginRight: 6 }} />
                    {isPlatform ? '所属平台' : '所属租户'}
                  </span>
                }
              >
                {isPlatform
                  ? '运营方'
                  : user?.tenant_name
                    ? `${user.tenant_name}（${user.tenant_code}）`
                    : '-'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>

        <Col xs={24} lg={16} xxl={18}>
          <Card bordered={false} style={{ borderRadius: 12 }}>
            <Tabs
              defaultActiveKey="profile"
              items={[
                {
                  key: 'profile',
                  label: (
                    <span>
                      <IdcardOutlined style={{ marginRight: 6 }} />
                      基本资料
                    </span>
                  ),
                  children: (
                    <Form<ProfileFormValues>
                      form={profileForm}
                      layout="vertical"
                      initialValues={{ nickname: user?.nickname ?? '' }}
                      onFinish={handleSaveProfile}
                      className="app-profile-form"
                    >
                      <Form.Item label="登录账号">
                        <Input value={user?.username ?? ''} disabled />
                      </Form.Item>
                      <Form.Item
                        name="nickname"
                        label="昵称"
                        rules={[
                          { required: true, message: '请输入昵称' },
                          { max: 64, message: '昵称最长 64 个字符' },
                          {
                            validator: (_rule, value: string) =>
                              value?.trim()
                                ? Promise.resolve()
                                : Promise.reject(new Error('昵称不能为空白字符')),
                          },
                        ]}
                        extra="昵称将展示在顶栏账号、水印与欢迎语中"
                      >
                        <Input placeholder="请输入昵称" maxLength={64} allowClear />
                      </Form.Item>
                      <Form.Item>
                        <Button type="primary" htmlType="submit" loading={savingProfile}>
                          保存修改
                        </Button>
                      </Form.Item>
                    </Form>
                  ),
                },
                {
                  key: 'password',
                  label: (
                    <span>
                      <LockOutlined style={{ marginRight: 6 }} />
                      修改密码
                    </span>
                  ),
                  children: (
                    <Form<PasswordFormValues>
                      form={passwordForm}
                      layout="vertical"
                      onFinish={handleChangePassword}
                      className="app-profile-form"
                      autoComplete="new-password"
                    >
                      <Form.Item
                        name="old_password"
                        label="原密码"
                        rules={[{ required: true, message: '请输入原密码' }]}
                      >
                        <Input.Password placeholder="请输入原密码" autoComplete="current-password" />
                      </Form.Item>
                      <Form.Item
                        name="new_password"
                        label="新密码"
                        rules={[
                          { required: true, message: '请输入新密码' },
                          { min: 6, message: '密码至少 6 位' },
                          { max: 64, message: '密码最长 64 位' },
                        ]}
                        extra="6-64 位，建议同时包含字母与数字"
                      >
                        <Input.Password placeholder="请输入新密码" autoComplete="new-password" />
                      </Form.Item>
                      <Form.Item
                        name="confirm_password"
                        label="确认新密码"
                        dependencies={['new_password']}
                        rules={[
                          { required: true, message: '请再次输入新密码' },
                          ({ getFieldValue }) => ({
                            validator(_rule, value: string) {
                              if (!value || getFieldValue('new_password') === value) {
                                return Promise.resolve()
                              }
                              return Promise.reject(new Error('两次输入的密码不一致'))
                            },
                          }),
                        ]}
                      >
                        <Input.Password placeholder="请再次输入新密码" autoComplete="new-password" />
                      </Form.Item>
                      <Form.Item>
                        <Button type="primary" htmlType="submit" loading={savingPassword}>
                          确认修改
                        </Button>
                      </Form.Item>
                    </Form>
                  ),
                },
              ]}
            />
          </Card>
        </Col>
      </Row>
    </PageContainer>
  )
}
