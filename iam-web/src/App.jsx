import { Routes, Route, Navigate } from 'react-router-dom'
import MainLayout from './layouts/MainLayout'
import UserManagement from './pages/UserManagement'
import RoleManagement from './pages/RoleManagement'
import PermissionManagement from './pages/PermissionManagement'
import Profile from './pages/Profile'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<MainLayout />}>
        <Route index element={<Navigate to="/users" replace />} />
        <Route path="users" element={<UserManagement />} />
        <Route path="roles" element={<RoleManagement />} />
        <Route path="permissions" element={<PermissionManagement />} />
        <Route path="profile" element={<Profile />} />
      </Route>
    </Routes>
  )
}