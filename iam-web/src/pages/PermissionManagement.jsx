import { useState } from 'react'
import {
  Table,
  Button,
  Space,
  Tag,
  Modal,
  Form,
  Input,
  Select,
  Popconfirm,
  message,
} from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'

const permissionTree = [
  {
    id: 1,
    key: 'user',
    title: '用户管理',
    code: 'user',
    type: '目录',
    method: '-',
    path: '/users',
    children: [
      { id: 11, key: 'user_view', title: '用户查看', code: 'user:view', type: '按钮', method: 'GET', path: '/api/users' },
      { id: 12, key: 'user_add', title: '用户新增', code: 'user:add', type: '按钮', method: 'POST', path: '/api/users' },
      { id: 13, key: 'user_edit', title: '用户编辑', code: 'user:edit', type: '按钮', method: 'PUT', path: '/api/users/:id' },
      { id: 14, key: 'user_delete', title: '用户删除', code: 'user:delete', type: '按钮', method: 'DELETE', path: '/api/users/:id' },
    ],
  },
  {
    id: 2,
    key: 'role',
    title: '角色管理',
    code: 'role',
    type: '目录',
    method: '-',
    path: '/roles',
    children: [
      { id: 21, key: 'role_view', title: '角色查看', code: 'role:view', type: '按钮', method: 'GET', path: '/api/roles' },
      { id: 22, key: 'role_add', title: '角色新增', code: 'role:add', type: '按钮', method: 'POST', path: '/api/roles' },
      { id: 23, key: 'role_edit', title: '角色编辑', code: 'role:edit', type: '按钮', method: 'PUT', path: '/api/roles/:id' },
      { id: 24, key: 'role_delete', title: '角色删除', code: 'role:delete', type: '按钮', method: 'DELETE', path: '/api/roles/:id' },
    ],
  },
  {
    id: 3,
    key: 'permission',
    title: '权限管理',
    code: 'permission',
    type: '目录',
    method: '-',
    path: '/permissions',
    children: [
      { id: 31, key: 'perm_view', title: '权限查看', code: 'permission:view', type: '按钮', method: 'GET', path: '/api/permissions' },
      { id: 32, key: 'perm_assign', title: '权限分配', code: 'permission:assign', type: '按钮', method: 'POST', path: '/api/permissions/assign' },
    ],
  },
]

const flattenData = (tree) => {
  const result = []
  const walk = (nodes, parent) => {
    nodes.forEach((node) => {
      result.push({ ...node, parentName: parent || '-' })
      if (node.children) walk(node.children, node.title)
    })
  }
  walk(tree, null)
  return result
}

const tableData = flattenData(permissionTree)

const methodColors = {
  GET: 'green',
  POST: 'blue',
  PUT: 'orange',
  DELETE: 'red',
  '-': 'default',
}

export default function PermissionManagement() {
  const [modalOpen, setModalOpen] = useState(false)
  const [editingPerm, setEditingPerm] = useState(null)
  const [form] = Form.useForm()

  const columns = [
    { title: '权限名称', dataIndex: 'title', key: 'title', width: 160 },
    { title: '上级', dataIndex: 'parentName', key: 'parentName', width: 120 },
    { title: '权限编码', dataIndex: 'code', key: 'code', width: 160, render: (v) => <Tag color="geekblue">{v}</Tag> },
    {
      title: '类型',
      dataIndex: 'type',
      key: 'type',
      width: 100,
      render: (v) => (v === '目录' ? <Tag color="purple">{v}</Tag> : <Tag>{v}</Tag>),
    },
    {
      title: '请求方式',
      dataIndex: 'method',
      key: 'method',
      width: 100,
      render: (v) => <Tag color={methodColors[v]}>{v}</Tag>,
    },
    { title: '接口路径', dataIndex: 'path', key: 'path' },
    {
      title: '操作',
      key: 'action',
      width: 160,
      render: (_, record) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => openEditModal(record)}>编辑</Button>
          <Popconfirm title="确定删除该权限？" onConfirm={() => message.success('删除成功')} okText="确定" cancelText="取消">
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const openAddModal = () => {
    setEditingPerm(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEditModal = (record) => {
    setEditingPerm(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const handleModalOk = () => {
    form.validateFields().then(() => {
      message.success(editingPerm ? '编辑成功' : '新增成功')
      setModalOpen(false)
    })
  }

  return (
    <div>
      <div style={{ marginBottom: 16, textAlign: 'right' }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={openAddModal}>
          新增权限
        </Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={tableData}
        pagination={false}
        expandable={{
          defaultExpandAllRows: true,
        }}
      />

      <Modal
        title={editingPerm ? '编辑权限' : '新增权限'}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText="确定"
        cancelText="取消"
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item name="title" label="权限名称" rules={[{ required: true, message: '请输入权限名称' }]}>
            <Input placeholder="请输入权限名称" />
          </Form.Item>
          <Form.Item name="code" label="权限编码" rules={[{ required: true, message: '请输入权限编码' }]}>
            <Input placeholder="例如 user:view" />
          </Form.Item>
          <Form.Item name="type" label="类型" rules={[{ required: true, message: '请选择类型' }]}>
            <Select
              placeholder="请选择类型"
              options={[
                { value: '目录', label: '目录' },
                { value: '菜单', label: '菜单' },
                { value: '按钮', label: '按钮' },
              ]}
            />
          </Form.Item>
          <Form.Item name="method" label="请求方式">
            <Select
              placeholder="请选择请求方式"
              allowClear
              options={[
                { value: 'GET', label: 'GET' },
                { value: 'POST', label: 'POST' },
                { value: 'PUT', label: 'PUT' },
                { value: 'DELETE', label: 'DELETE' },
              ]}
            />
          </Form.Item>
          <Form.Item name="path" label="接口路径">
            <Input placeholder="例如 /api/users" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  )
}