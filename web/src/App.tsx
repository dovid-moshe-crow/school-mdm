import { App as AntApp, ConfigProvider } from 'antd'
import heIL from 'antd/locale/he_IL'
import { Route, Routes } from 'react-router-dom'
import { theme } from './theme'
import Home from './pages/Home'
import Portal from './pages/Portal'
import Admin from './pages/Admin'

export default function App() {
  return (
    <ConfigProvider direction="rtl" locale={heIL} theme={theme}>
      <AntApp>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/d/:deviceId" element={<Portal />} />
          <Route path="/admin" element={<Admin />} />
        </Routes>
      </AntApp>
    </ConfigProvider>
  )
}
