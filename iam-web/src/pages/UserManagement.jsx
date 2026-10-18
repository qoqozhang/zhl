import { useState } from 'react'
import {
  Layout,
  Tree,
  Table,
  Button,
  Input,
  Space,
  Tag,
  Modal,
  Form,
  Select,
  Switch,
  Popconfirm,
  message,
} from 'antd'
import {
  PlusOutlined,
  SearchOutlined,
  ReloadOutlined,
  EditOutlined,
  DeleteOutlined,
  UserOutlined,
  MailOutlined,
  PhoneOutlined,
} from '@ant-design/icons'

const { Sider, Content } = Layout

const deptTreeData = [
  {
    title: '总公司',
    key: 'all',
    children: [
      {
        title: '技术部',
        key: 'tech',
        children: [
          { title: '前端组', key: 'frontend' },
          { title: '后端组', key: 'backend' },
          { title: '测试组', key: 'qa' },
        ],
      },
      {
        title: '产品部',
        key: 'product',
        children: [
          { title: '产品一组', key: 'p1' },
          { title: '产品二组', key: 'p2' },
        ],
      },
      {
        title: '市场部',
        key: 'market',
      },
      {
        title: '人事部',
        key: 'hr',
      },
    ],
  },
]

const allUsers = [
  { id: 1, username: 'zhangsan', name: '张三', email: 'zhangsan@example.com', phone: '13800138001', department: 'tech', deptName: '技术部', role: '管理员', status: true, createdAt: '2025-01-15' },
  { id: 2, username: 'lisi', name: '李四', email: 'lisi@example.com', phone: '13800138002', department: 'frontend', deptName: '前端组', role: '普通用户', status: true, createdAt: '2025-02-20' },
  { id: 3, username: 'wangwu', name: '王五', email: 'wangwu@example.com', phone: '13800138003', department: 'backend', deptName: '后端组', role: '普通用户', status: false, createdAt: '2025-03-10' },
  { id: 4, username: 'zhaoliu', name: '赵六', email: 'zhaoliu@example.com', phone: '13800138004', department: 'product', deptName: '产品部', role: '产品经理', status: true, createdAt: '2025-04-05' },
  { id: 5, username: 'sunqi', name: '孙七', email: 'sunqi@example.com', phone: '13800138005', department: 'market', deptName: '市场部', role: '普通用户', status: true, createdAt: '2025-05-12' },
  { id: 6, username: 'zhouba', name: '周八', email: 'zhouba@example.com', phone: '13800138006', department: 'hr', deptName: '人事部', role: 'HR', status: true, createdAt: '2025-06-18' },
  { id: 7, username: 'wujiu', name: '吴九', email: 'wujiu@example.com', phone: '13800138007', department: 'qa', deptName: '测试组', role: '测试工程师', status: true, createdAt: '2025-07-22' },
  { id: 8, username: 'zhengshi', name: '郑十', email: 'zhengshi@example.com', phone: '13800138008', department: 'p1', deptName: '产品一组', role: '普通用户', status: false, createdAt: '2025-08-30' },
]

const deptMap = {
  all: ['tech', 'product', 'market', 'hr', 'frontend', 'backend', 'qa', 'p1', 'p2'],
  tech: ['frontend', 'backend', 'qa'],
  product: ['p1', 'p2'],
  market: ['market'],
  hr: ['hr'],
  frontend: ['frontend'],
  backend: ['backend'],
  qa: ['qa'],
  p1: ['p1'],
  p2: ['p2'],
}

export default function UserManagement() {
  const [selectedDept, setSelectedDept] = useState('all')
  const [searchText, setSearchText] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editingUser, setEditingUser] = useState(null)
  const [form] = Form.useForm()

  const filteredUsers = allUsers.filter((u) => {
    const deptKeys = deptMap[selectedDept] || [selectedDept]
    const matchDept = selectedDept === 'all' || deptKeys.includes(u.department)
    const matchSearch =
      !searchText ||
      u.username.includes(searchText) ||
      u.name.includes(searchText) ||
      u.email.includes(searchText)
    return matchDept && matchSearch
  })

  const handleTreeSelect = (keys) => {
    if (keys.length > 0) setSelectedDept(keys[0])
  }

  const openAddModal = () => {
    setEditingUser(null)
    form.resetFields()
    form.setFieldsValue({ status: true })
    setModalOpen(true)
  }

  const openEditModal = (record) => {
    setEditingUser(record)
    form.setFieldsValue(record)
    setModalOpen(true)
  }

  const handleModalOk = () => {
    form.validateFields().then(() => {
      message.success(editingUser ? '编辑成功' : '新增成功')
      setModalOpen(false)
    })
  }

  const handleDelete = () => {
    message.success('删除成功')
  }

  const columns = [
    {
      title: '用户名',
      dataIndex: 'username',
      key: 'username',
      render: (v) => (
        <Space>
          <UserOutlined />
          <a>{v}</a>
        </Space>
      ),
    },
    { title: '姓名', dataIndex: 'name', key: 'name' },
    {
      title: '邮箱',
      dataIndex: 'email',
      key: 'email',
      render: (v) => (
        <Space>
          <MailOutlined />
          {v}
        </Space>
      ),
    },
    {
      title: '手机号',
      dataIndex: 'phone',
      key: 'phone',
      render: (v) => (
        <Space>
          <PhoneOutlined />
          {v}
        </Space>
      ),
    },
    { title: '所属部门', dataIndex: 'deptName', key: 'deptName' },
    {
      title: '角色',
      dataIndex: 'role',
      key: 'role',
      render: (v) => <Tag color="blue">{v}</Tag>,
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (v) =>
        v ? <Tag color="success">启用</Tag> : <Tag color="default">禁用</Tag>,
    },
    { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt' },
    {
      title: '操作',
      key: 'action',
      render: (_, record) => (
        <Space>
          <Button type="link" icon={<EditOutlined />} onClick={() => openEditModal(record)}>
            编辑
          </Button>
          <Popconfirm title="确定删除该用户？" onConfirm={handleDelete} okText="确定" cancelText="取消">
            <Button type="link" danger icon={<DeleteOutlined />}>
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <Layout style={{ background: '#fff', minHeight: 'calc(100vh - 120px)' }}>
      <Sider width={240} style={{ background: '#fff', borderRight: '1px solid #f0f0f0', padding: 16 }}>
        <div style={{ marginBottom: 12, fontWeight: 500 }}>部门架构</div>
        <Tree
          showLine
          defaultExpandAll
          treeData={deptTreeData}
          selectedKeys={[selectedDept]}
          onSelect={handleTreeSelect}
        />
      </Sider>
      <Content style={{ padding: '0 16px' }}>
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
          <Space>
            <Input
              placeholder="搜索用户名/姓名/邮箱"
              prefix={<SearchOutlined />}
              value={searchText}
              onChange={(e) => setSearchText(e.target.value)}
              style={{ width: 260 }}
              allowClear
            />
            <Button icon={<ReloadOutlined />} onClick={() => setSearchText('')}>
              重置
            </Button>
          </Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={openAddModal}>
            新增用户
          </Button>
        </div>

        <Table
          rowKey="id"
          columns={columns}
          dataSource={filteredUsers}
          pagination={{ pageSize: 10, showTotal: (total) => `共 ${total} 条` }}
        />
      </Content>

      <Modal
        title={editingUser ? '编辑用户' : '新增用户'}
        open={modalOpen}
        onOk={handleModalOk}
        onCancel={() => setModalOpen(false)}
        okText="确定"
        cancelText="取消"
        width={560}
      >
        <Form form={form} layout="vertical" style={{ marginTop: 16 }}>
          <Form.Item
            name="username"
            label="用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="请输入用户名" />
          </Form.Item>
          <Form.Item
            name="name"
            label="姓名"
            rules={[{ required: true, message: '请输入姓名' }]}
          >
            <Input placeholder="请输入姓名" />
          </Form.Item>
          <Form.Item name="email" label="邮箱">
            <Input placeholder="请输入邮箱" />
          </Form.Item>
          <Form.Item name="phone" label="手机号">
            <Input placeholder="请输入手机号" />
          </Form.Item>
          <Form.Item name="role" label="角色">
            <Select
              placeholder="请选择角色"
              options={[
                { value: '管理员', label: '管理员' },
                { value: '普通用户', label: '普通用户' },
                { value: '产品经理', label: '产品经理' },
                { value: 'HR', label: 'HR' },
                { value: '测试工程师', label: '测试工程师' },
              ]}
            />
          </Form.Item>
          <Form.Item name="status" label="状态" valuePropName="checked">
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>
          <Form.Item
            name="password"
            label={editingUser ? '重置密码' : '密码'}
            rules={
              editingUser
                ? [{ min: 6, message: '密码长度不能少于 6 位' }]
                : [
                    { required: true, message: '请输入密码' },
                    { min: 6, message: '密码长度不能少于 6 位' },
                  ]
            }
          >
            <Input.Password
              placeholder={editingUser ? '留空则不修改密码' : '请输入密码（至少 6 位）'}
            />
          </Form.Item>
          <Form.Item
            name="confirmPassword"
            label="确认密码"
            dependencies={['password']}
            rules={[
              editingUser
                ? {
                    validator: (_, value) =>
                      !value || value === form.getFieldValue('password')
                        ? Promise.resolve()
                        : Promise.reject(new Error('两次输入的密码不一致')),
                  }
                : {
                    required: true,
                    validator: (_, value) =>
                      value === form.getFieldValue('password')
                        ? Promise.resolve()
                        : Promise.reject(new Error('两次输入的密码不一致')),
                  },
            ]}
          >
            <Input.Password placeholder="请再次输入密码" />
          </Form.Item>
        </Form>
      </Modal>
    </Layout>
  )
}