import { Typography } from 'antd'
import { Link } from 'react-router-dom'

/** Public support page for App Store / Custom App listing. */
export default function Support() {
  return (
    <div className="page-shell legal-page" dir="ltr">
      <Typography.Title level={2}>KFilter — Support</Typography.Title>
      <Typography.Paragraph type="secondary">School companion app support</Typography.Paragraph>

      <Typography.Paragraph>
        KFilter opens your school’s student portal on managed iPhones and can show notifications
        about request updates. It is provided privately to schools through Apple School Manager.
      </Typography.Paragraph>

      <Typography.Title level={4}>Students / device users</Typography.Title>
      <Typography.Paragraph>
        If the portal does not load, notifications fail, or you need a device ID changed, contact
        your <strong>school IT / administration</strong> first. They manage enrollment and the
        school server.
      </Typography.Paragraph>

      <Typography.Title level={4}>School administrators</Typography.Title>
      <ul>
        <li>
          Portal / admin UI: <Typography.Text code>https://nanok.kfilter.net/admin</Typography.Text>
        </li>
        <li>
          Under MDM, send Managed App Config with <Typography.Text code>enrollment_id</Typography.Text>{' '}
          (and optional <Typography.Text code>portal_base_url</Typography.Text>).
        </li>
        <li>
          For TestFlight / non-MDM installs, enter the device enrollment id on first launch.
        </li>
      </ul>

      <Typography.Title level={4}>App Review / technical contact</Typography.Title>
      <Typography.Paragraph>
        For App Store review questions about this Custom App build, use the contact details provided
        in App Store Connect for this submission, or email the developer account associated with the
        app listing.
      </Typography.Paragraph>
      <Typography.Paragraph>
        Demo portal base: <Typography.Text code>https://nanok.kfilter.net</Typography.Text>
      </Typography.Paragraph>

      <Typography.Title level={4}>Privacy</Typography.Title>
      <Typography.Paragraph>
        See our <Link to="/privacy">Privacy Policy</Link>.
      </Typography.Paragraph>

      <Typography.Paragraph>
        <Link to="/">Back to home</Link>
      </Typography.Paragraph>
    </div>
  )
}
