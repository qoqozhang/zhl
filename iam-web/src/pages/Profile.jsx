import { useState } from 'react'
import {
  Card,
  Descriptions,
  Avatar,
  Button,
  Modal,
  Form,
  Input,
  Tag,
  Space,
  message,
  Divider,
} from 'antd'
import {
  UserOutlined,
  MailOutlined,
  PhoneOutlined,
  IdcardOutlined,
  SafetyOutlined,
  ClusterOutlined,
  CalendarOutlined,
  KeyOutlined,
  EditOutlined,
} from '@ant-design/icons'

const currentUser = {
  username: 'admin',
  name: '系统管理员',
  email: 'admin@example.com',
  phone: '13800138000',
  department: '总公司 / 技术部',
  role: '超级管理员',
  status: '启用',
  lastLogin: '2026-10-18 09:30:00',
  createdAt: '2025-01-01 00:00:00',
}

export default function Profile() {
  const [pwdModalOpen, setPwdModalOpen] = useState(false)
  const [form] = Form.useForm()

  const handlePwdOk = () => {
    form.validateFields().then((values) => {
      if (values.newPassword !== values.confirmPassword) {
        message.error('两次输入的密码不一致')
        return
      }
      message.success('密码修改成功')
      setPwdModalOpen(false)
      form.resetFields()
    })
  }

  return (
    <div style={{ maxWidth: 800, margin: '0 auto' }}>
      <Card style={{ marginBottom: 24 }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 24 }}>
          <Avatar size={96} icon={<UserOutlined />} style={{ backgroundColor: '#1677ff', flexShrink: 0 }} />
          <div>
            <h2 style={{ margin: '0 0 8px' }}>{currentUser.name}</h2>
            <div style={{ color: '#888', marginBottom: 8 }}>
              <Space>
                <Tag color="blue">{currentUser.role}</Tag>
                <Tag color="green">{currentUser.status}</Tag>
              </Space>
            </div>
            <div style={{ color: '#666' }}>@{currentUser.username}</div>
          </div>
        </div>
      </Card>

      <Card
        title="基本信息"
        extra={
          <Space>
            <Button icon={<EditOutlined />}>编辑资料</Button>
            <Button icon={<KeyOutlined />} onClick={() => setPwdModalOpen(true)}>
              修改密码
            </Button>
          </Space>
        }
      >
        <Descriptions column={2} bordered>
          <Descriptions.Item label="用户名">
            <Space><UserOutlined />{currentUser.username}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="姓名">
            <Space><IdcardOutlined />{currentUser.name}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="邮箱">
            <Space><MailOutlined />{currentUser.email}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="手机号">
            <Space><PhoneOutlined />{currentUser.phone}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="所属部门">
            <Space><ClusterOutlined />{currentUser.department}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="角色">
            <Space><SafetyOutlined />{currentUser.role}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="上次登录">
            <Space><CalendarOutlined />{currentUser.lastLogin}</Space>
          </Descriptions.Item>
          <Descriptions.Item label="创建时间">
            <Space><CalendarOutlined />{currentUser.createdAt}</Space>
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Divider />

      <Card title="安全设置" size="small">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '12px 0' }}>
          <div>
            <div style={{ fontWeight: 500 }}>登录密码</div>
            <div style={{ color: '#888', fontSize: 13 }}>建议定期修改密码，确保账户安全</div>
          </div>
          <Button icon={<KeyOutlined />} onClick={() => setPwdModalOpen(true)}>
            修改密码
          </Button>
        </div>
      </Card>

      <Modal
        title="修改密码"
        open={pwdModalOpen}
        onOk={handlePwdOk}
        onCancel={() => setPwdModalOpen(false)}
        okText="确定"
        cancelText="取消"
        width={480}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="oldPassword"
            label="当前密码"
            rules={[{ required: true, message: '请输入当前密码' }]}
          >
            <Input.Password placeholder="请输入当前密码" />
          </Form.Item>
          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码长度不能少于 6 位' },
            ]}
          >
            <Input.Password placeholder="请输入新密码（至少 6 位）" />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            rules={[{ required: true, message: '请再次输入新密码' }]}
          >
            <Input.Password placeholder="请再次输入新密码" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}