import { useState } from 'react'
import {
  Table,
  Input,
  Select,
  Button,
  Space,
  Tag,
  DatePicker,
  Descriptions,
  Modal,
} from 'antd'
import { SearchOutlined, ReloadOutlined, FileTextOutlined } from '@ant-design/icons'

const { RangePicker } = DatePicker

const operationLogs = [
  { id: 1, userId: 1, username: 'zhangsan', name: '张三', action: '新增用户', module: '用户管理', method: 'POST', ip: '192.168.1.100', status: '成功', cost: 128, time: '2026-10-19 09:15:32' },
  { id: 2, userId: 2, username: 'lisi', name: '李四', action: '编辑角色', module: '角色管理', method: 'PUT', ip: '192.168.1.101', status: '成功', cost: 96, time: '2026-10-19 09:12:18' },
  { id: 3, userId: 1, username: 'zhangsan', name: '张三', action: '删除用户', module: '用户管理', method: 'DELETE', ip: '192.168.1.100', status: '成功', cost: 75, time: '2026-10-19 09:08:45' },
  { id: 4, userId: 3, username: 'wangwu', name: '王五', action: '分配权限', module: '权限管理', method: 'POST', ip: '192.168.1.102', status: '失败', cost: 2103, time: '2026-10-19 09:05:11' },
  { id: 5, userId: 1, username: 'zhangsan', name: '张三', action: '用户登录', module: '认证', method: 'POST', ip: '192.168.1.100', status: '成功', cost: 356, time: '2026-10-19 09:00:02' },
  { id: 6, userId: 4, username: 'zhaoliu', name: '赵六', action: '重置密码', module: '用户管理', method: 'PUT', ip: '192.168.1.103', status: '成功', cost: 187, time: '2026-10-18 17:42:28' },
  { id: 7, userId: 1, username: 'zhangsan', name: '张三', action: '查询日志', module: '操作日志', method: 'GET', ip: '192.168.1.100', status: '成功', cost: 42, time: '2026-10-18 17:38:55' },
  { id: 8, userId: 5, username: 'sunqi', name: '孙七', action: '新增权限', module: '权限管理', method: 'POST', ip: '192.168.1.104', status: '成功', cost: 134, time: '2026-10-18 16:55:12' },
  { id: 9, userId: 2, username: 'lisi', name: '李四', action: '修改状态', module: '用户管理', method: 'PUT', ip: '192.168.1.101', status: '成功', cost: 68, time: '2026-10-18 15:20:33' },
  { id: 10, userId: 6, username: 'zhouba', name: '周八', action: '用户登录', module: '认证', method: 'POST', ip: '10.0.0.55', status: '失败', cost: 1823, time: '2026-10-18 14:11:09' },
]

const methodColors = {
  GET: 'green',
  POST: 'blue',
  PUT: 'orange',
  DELETE: 'red',
}

export default function OperationLog() {
  const [detailOpen, setDetailOpen] = useState(false)
  const [detailRecord, setDetailRecord] = useState(null)
  const [searchText, setSearchText] = useState('')
  const [statusFilter, setStatusFilter] = useState(undefined)

  const filtered = operationLogs.filter((log) => {
    const matchText =
      !searchText ||
      log.username.includes(searchText) ||
      log.name.includes(searchText) ||
      log.action.includes(searchText)
    const matchStatus = !statusFilter || log.status === statusFilter
    return matchText && matchStatus
  })

  const columns = [
    { title: '操作人', dataIndex: 'name', key: 'name', width: 110, render: (v) => <Space><FileTextOutlined />{v}</Space> },
    { title: '用户名', dataIndex: 'username', key: 'username', width: 120 },
    { title: '操作', dataIndex: 'action', key: 'action', width: 130 },
    { title: '模块', dataIndex: 'module', key: 'module', width: 120, render: (v) => <Tag>{v}</Tag> },
    { title: '方法', dataIndex: 'method', key: 'method', width: 90, render: (v) => <Tag color={methodColors[v]}>{v}</Tag> },
    { title: 'IP 地址', dataIndex: 'ip', key: 'ip', width: 130 },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      width: 90,
      render: (v) => (v === '成功' ? <Tag color="success">{v}</Tag> : <Tag color="error">{v}</Tag>),
    },
    {
      title: '耗时',
      dataIndex: 'cost',
      key: 'cost',
      width: 100,
      sorter: (a, b) => a.cost - b.cost,
      render: (v) => <span>{v} ms</span>,
    },
    { title: '时间', dataIndex: 'time', key: 'time', width: 180, sorter: (a, b) => a.time.localeCompare(b.time) },
    {
      title: '操作',
      key: 'action',
      width: 80,
      fixed: 'right',
      render: (_, record) => (
        <Button type="link" onClick={() => { setDetailRecord(record); setDetailOpen(true) }}>详情</Button>
      ),
    },
  ]

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', gap: 12, flexWrap: 'wrap' }}>
        <Input
          placeholder="搜索操作人 / 操作"
          prefix={<SearchOutlined />}
          value={searchText}
          onChange={(e) => setSearchText(e.target.value)}
          style={{ width: 220 }}
          allowClear
        />
        <Select
          placeholder="状态"
          value={statusFilter}
          onChange={setStatusFilter}
          style={{ width: 120 }}
          allowClear
          options={[
            { value: '成功', label: '成功' },
            { value: '失败', label: '失败' },
          ]}
        />
        <RangePicker showTime />
        <Button icon={<ReloadOutlined />} onClick={() => { setSearchText(''); setStatusFilter(undefined) }}>重置</Button>
      </div>

      <Table
        rowKey="id"
        columns={columns}
        dataSource={filtered}
        scroll={{ x: 1200 }}
        pagination={{ pageSize: 10, showTotal: (total) => `共 ${total} 条` }}
      />

      <Modal
        title="操作日志详情"
        open={detailOpen}
        onCancel={() => setDetailOpen(false)}
        footer={null}
        width={560}
      >
        {detailRecord && (
          <Descriptions column={1} bordered size="small" style={{ marginTop: 12 }}>
            <Descriptions.Item label="日志 ID">{detailRecord.id}</Descriptions.Item>
            <Descriptions.Item label="操作人">{detailRecord.name}</Descriptions.Item>
            <Descriptions.Item label="用户名">{detailRecord.username}</Descriptions.Item>
            <Descriptions.Item label="操作">{detailRecord.action}</Descriptions.Item>
            <Descriptions.Item label="所属模块">{detailRecord.module}</Descriptions.Item>
            <Descriptions.Item label="请求方法">{detailRecord.method}</Descriptions.Item>
            <Descriptions.Item label="IP 地址">{detailRecord.ip}</Descriptions.Item>
            <Descriptions.Item label="状态">{detailRecord.status}</Descriptions.Item>
            <Descriptions.Item label="耗时">{detailRecord.cost} ms</Descriptions.Item>
            <Descriptions.Item label="操作时间">{detailRecord.time}</Descriptions.Item>
          </Descriptions>
        )}
      </Modal>
    </div>
  )
}