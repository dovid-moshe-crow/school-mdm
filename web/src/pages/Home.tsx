import { Link } from 'react-router-dom'
import { he } from '../he'

export default function Home() {
  return (
    <main className="shell">
      <h1 className="brand">{he.brand}</h1>
      <p className="lede">{he.homeLead}</p>
      <div className="panel">
        <p className="meta">
          כתובת המכשיר נראית כך: <code>/d/&lt;מזהה-מכשיר&gt;</code>
        </p>
        <p>
          <Link to="/admin">{he.openAdmin}</Link>
        </p>
      </div>
    </main>
  )
}
