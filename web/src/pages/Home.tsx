import { Button, Card, Space, Typography } from 'antd'
import { Link } from 'react-router-dom'
import { he } from '../he'

export default function Home() {
  return (
    <div className="page-shell">
      <Space direction="vertical" size="middle" style={{ width: '100%' }}>
        <div>
          <Typography.Title level={2} style={{ marginBottom: 8 }}>
            {he.brand}
          </Typography.Title>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
            {he.homeLead}
          </Typography.Paragraph>
        </div>
        <Card>
          <Typography.Paragraph type="secondary" style={{ marginBottom: 16 }}>
            פורטל תלמיד: <Typography.Text code>/d/&lt;מזהה-מכשיר&gt;</Typography.Text>
          </Typography.Paragraph>
          <Link to="/admin">
            <Button type="primary">{he.openAdmin}</Button>
          </Link>
        </Card>
      </Space>
    </div>
  )
}
