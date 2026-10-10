import { useState } from 'react'
import { App, Form, Input, Modal, Typography } from 'antd'
import { useAuthStore } from '@/store/auth'
import { lockOwner, useScreenLockStore } from '@/store/screenLock'

interface LockPasswordModalProps {
  open: boolean
  lockOnSave: boolean
  onClose: () => void
}

/** 密码输入仅在弹窗内保留，关闭后销毁；持久化层只存加盐摘要。 */
export function LockPasswordModal({ open, lockOnSave, onClose }: LockPasswordModalProps) {
  const [form] = Form.useForm()
  const [saving, setSaving] = useState(false)
  const { message } = App.useApp()
  const owner = lockOwner(useAuthStore((s) => s.user))
  const configured = useScreenLockStore((s) => Boolean(s.accounts[owner]))

  const save = async () => {
    try {
      const values = await form.validateFields()
      setSaving(true)
      await useScreenLockStore
        .getState()
        .setPassword(owner, values.password, values.currentPassword)
      if (lockOwner(useAuthStore.getState().user) !== owner) return
      if (lockOnSave) useScreenLockStore.getState().lock(owner)
      message.success(lockOnSave ? '密码已设置，屏幕已锁定' : '锁屏密码已保存')
      onClose()
    } catch (error) {
      if (error instanceof Error) message.error(error.message)
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      title={configured ? '修改锁屏密码' : '设置锁屏密码'}
      open={open}
      onCancel={onClose}
      onOk={save}
      okText={lockOnSave ? '保存并锁屏' : '保存密码'}
      cancelText="取消"
      confirmLoading={saving}
      cancelButtonProps={{ disabled: saving }}
      closable={!saving}
      maskClosable={false}
      keyboard={!saving}
      destroyOnHidden
      afterClose={() => form.resetFields()}
    >
      <Typography.Paragraph type="secondary">
        离开工位时可点击工具栏的锁屏按钮，输入此密码后继续操作。
      </Typography.Paragraph>
      <Form form={form} layout="vertical" preserve={false} onFinish={save} disabled={saving}>
        {configured && (
          <Form.Item
            label="当前锁屏密码"
            name="currentPassword"
            rules={[{ required: true, message: '请输入当前锁屏密码' }]}
          >
            <Input.Password autoComplete="current-password" maxLength={64} />
          </Form.Item>
        )}
        <Form.Item
          label="新锁屏密码"
          name="password"
          rules={[
            { required: true, message: '请输入锁屏密码' },
            { min: 6, max: 64, message: '密码长度为 6–64 位' },
            { whitespace: true, message: '密码不能全为空格' },
          ]}
        >
          <Input.Password
            autoComplete="new-password"
            maxLength={64}
            placeholder="输入 6–64 位自定义密码"
          />
        </Form.Item>
        <Form.Item
          label="确认新密码"
          name="confirmPassword"
          dependencies={['password']}
          rules={[
            { required: true, message: '请再次输入锁屏密码' },
            ({ getFieldValue }) => ({
              validator: (_, value) =>
                !value || value === getFieldValue('password')
                  ? Promise.resolve()
                  : Promise.reject(new Error('两次输入的密码不一致')),
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" maxLength={64} />
        </Form.Item>
      </Form>
    </Modal>
  )
}
