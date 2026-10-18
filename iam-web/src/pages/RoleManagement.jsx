import { useState } from 'react'
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Switch,
  Popconfirm,
  message,
  Transfer,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'

const roleList = [
  { id: 1, name: '超级管理员', code: 'super_admin', description: '拥有系统所有权限', userCount: 2, status: true, createdAt: '2025-01-10' },
  { id: 2, name: '普通管理员', code: 'admin', description: '拥有大部分管理权限', userCount: 5, status: true, createdAt: '2025-02-15' },
  { id: 3, name: '普通用户', code: 'user', description: '基础操作权限', userCount: 12, status: true, createdAt: '2025-03-20' },
  { id: 4, name: '访客', code: 'guest', description: '只读权限', userCount: 3, status: false, createdAt: '2025-04-25' },
]

const allPermissions = [
  { key: '1', title: '用户查看' },
  { key: '2', title: '用户新增' },
  { key: '3', title: '用户编辑' },
  { key: '4', title: '用户删除' },
  { key: '5', title: '角色查看' },
  { key: '6', title: '角色新增' },
  { key: '7', title: '角色编辑' },
  { key: '8', title: '角色删除' },
  { key: '9', title: '权限查看' },
  { key: '10', title: '权限分配' },
]

export default function RoleManagement() {
  const [modalOpen, setModalOpen] = useState(false)
  const [permModalOpen, setPermModalOpen] = useState(false)
  const [editingRole, setEditingRole] = useState(null)
  const [targetKeys, setTargetKeys] = useState([])
  const [form] = Form.useForm()

  const columns = [
    { title: '角色名称', dataIndex: 'name', key: 'name' },
    { title: '角色编码', dataIndex: 'code', key: 'code', render: (v) => <Tag color="geekblue">{v}</Tag> },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { title: '用户数', dataIndex: 'userCount', key: 'userCount' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (v) => (v ? <Tag color="success">启用</Tag> : <Tag color="default">禁用</Tag>),
    },
    { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt' },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button type="link" onClick={() => openPermModal(record)}>权限配置</Button>
          <Button type="link" icon={<EditOutlined />} onClick={() => openEditModal(record)}>编辑</Button>
          <Popconfirm title="确定删除该角色？" onConfirm={() => message.success('删除成功')} okText="确定" cancelText="取消">
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const openAddModal = () => {
    setEditingRole(null)
    form.resetFields()
    form.setFieldsValue({ status: true })
    setModalOpen(true)
  }

  const openEditModal = (record) => {
    setEditingRole(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const openPermModal = (record) => {
    setEditingRole(record)
    setTargetKeys(['1', '2', '5', '9'])
    setPermModalOpen(true)
  }

  const handleModalOk = () => {
    form.validateFields().then(() => {
      message.success(editingRole ? '编辑成功' : '新增成功')
      setModalOpen(false)
    })
  }

  const handlePermOk = () => {
    message.success('权限配置成功')
    setPermModalOpen(false)
  }

  return (
    <div>
      <div style={{ marginBottom: 16, textAlign: 'right' }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openAddModal}>
          新增角色
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={roleList}
        pagination={{ pageSize: 10, showTotal: (total) => `共 ${total} 条` }}
      />

      <Modal
        title={editingRole ? '编辑角色' : '新增角色'}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText="确定"
        cancelText="取消"
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="name" label="角色名称" rules={[{ required: true, message: '请输入角色名称' }]}>
            <Input placeholder="请输入角色名称" />
          </Form.Item>
          <Form.Item name="code" label="角色编码" rules={[{ required: true, message: '请输入角色编码' }]}>
            <Input placeholder="请输入角色编码" />
          </Form.Item>
          <Form.Item name="description" label="描述">
            <Input.TextArea rows={3} placeholder="请输入描述" />
          </Form.Item>
          <Form.Item name="status" label="状态" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`权限配置 - ${editingRole?.name || ''}`}
        open={permModalOpen}
        onOk={handlePermOk}
        onCancel={() => setPermModalOpen(false)}
        okText="确定"
        cancelText="取消"
        width={640}
      >
        <Transfer
          dataSource={allPermissions}
          titles={['待分配权限', '已分配权限']}
          targetKeys={targetKeys}
          onChange={setTargetKeys}
          render={(item) => item.title}
          rowKey={(item) => item.key}
          listStyle={{ width: 250, height: 360 }}
        />
      </Modal>
    </div>
  )
}