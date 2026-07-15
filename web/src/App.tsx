import { Route, Routes } from 'react-router-dom'
import Home from './pages/Home'
import Portal from './pages/Portal'
import Admin from './pages/Admin'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<Home />} />
      <Route path="/d/:deviceId" element={<Portal />} />
      <Route path="/admin" element={<Admin />} />
    </Routes>
  )
}
