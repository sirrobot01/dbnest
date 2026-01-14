import { Routes, Route } from 'react-router-dom'
import { AuthGuard } from '@/components/AuthGuard'
import Dashboard from '@/pages/Dashboard'
import Databases from '@/pages/Databases'
import DatabaseDetail from '@/pages/DatabaseDetail'
import Topology from '@/pages/Topology'
import Backups from '@/pages/Backups'
import Login from '@/pages/Login'

function App() {
    return (
        <AuthGuard>
            <Routes>
                <Route path="/" element={<Dashboard />} />
                <Route path="/databases" element={<Databases />} />
                <Route path="/databases/:id" element={<DatabaseDetail />} />
                <Route path="/topology" element={<Topology />} />
                <Route path="/backups" element={<Backups />} />
                <Route path="/login" element={<Login />} />
            </Routes>
        </AuthGuard>
    )
}

export default App
