import { lazy, Suspense } from 'react'
import { App as AntApp, ConfigProvider } from 'antd'
import heIL from 'antd/locale/he_IL'
import { Route, Routes } from 'react-router-dom'
import { theme } from './theme'
import Home from './pages/Home'
import Portal from './pages/Portal'
import Admin from './pages/Admin'
import DeviceAdmin from './pages/DeviceAdmin'
import Privacy from './pages/Privacy'
import Support from './pages/Support'

const ApiDocs = lazy(() => import('./pages/ApiDocs'))

export default function App() {
  return (
    <ConfigProvider direction="rtl" locale={heIL} theme={theme}>
      <AntApp>
        <Suspense fallback={null}>
          <Routes>
            <Route path="/" element={<Home />} />
            <Route path="/privacy" element={<Privacy />} />
            <Route path="/support" element={<Support />} />
            <Route path="/d/:deviceId" element={<Portal />} />
            <Route path="/d/:deviceId/store" element={<Portal />} />
            <Route path="/admin" element={<Admin />} />
            <Route path="/admin/devices/:deviceId" element={<DeviceAdmin />} />
            <Route path="/api-docs" element={<ApiDocs />} />
          </Routes>
        </Suspense>
      </AntApp>
    </ConfigProvider>
  )
}
