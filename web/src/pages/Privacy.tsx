import { Typography } from 'antd'
import { Link } from 'react-router-dom'

/** Public privacy policy for App Store / Custom App listing. */
export default function Privacy() {
  return (
    <div className="page-shell legal-page" dir="ltr">
      <Typography.Title level={2}>KFilter — Privacy Policy</Typography.Title>
      <Typography.Paragraph type="secondary">Last updated: August 12, 2026</Typography.Paragraph>

      <Typography.Paragraph>
        KFilter is a school companion app that opens your school’s student portal and can deliver
        notifications about request updates. It is distributed privately to schools (Custom App /
        Apple School Manager), not as a general consumer App Store product.
      </Typography.Paragraph>

      <Typography.Title level={4}>Who is responsible</Typography.Title>
      <Typography.Paragraph>
        Your school (the organization that enrolls the device and runs the school portal) is the
        data controller for student portal use. The app developer provides the KFilter shell that
        connects to the school’s server (for example <Typography.Text code>nanok.kfilter.net</Typography.Text>
        ).
      </Typography.Paragraph>

      <Typography.Title level={4}>What we collect</Typography.Title>
      <Typography.Paragraph>
        Depending on how the school configures the service, the following may be processed:
      </Typography.Paragraph>
      <ul>
        <li>
          <strong>Device enrollment identifier</strong> — to open the correct student portal and bind
          the device under school management (MDM Managed App Config and/or manual entry).
        </li>
        <li>
          <strong>Push notification token</strong> — so the school server can send notifications about
          request status or school messages.
        </li>
        <li>
          <strong>Portal activity</strong> — requests, messages, and allow-list related data that you
          submit or that administrators manage in the school portal (stored on the school server).
        </li>
      </ul>

      <Typography.Title level={4}>How we use information</Typography.Title>
      <Typography.Paragraph>
        Information is used only to operate the school portal experience: identify the device,
        show the right content, notify students of updates, and let school staff manage devices and
        requests. We do not sell personal information. We do not use student data for advertising.
      </Typography.Paragraph>

      <Typography.Title level={4}>Sharing</Typography.Title>
      <Typography.Paragraph>
        Data stays with the school’s systems and necessary infrastructure providers (hosting, Apple
        push delivery via Expo/APNs). It is not shared with third parties for marketing.
      </Typography.Paragraph>

      <Typography.Title level={4}>Retention</Typography.Title>
      <Typography.Paragraph>
        Retention follows the school’s policies and the lifetime of device enrollment / portal
        records. Push tokens are kept while the app is registered for notifications.
      </Typography.Paragraph>

      <Typography.Title level={4}>Children</Typography.Title>
      <Typography.Paragraph>
        The app is intended for use on school-managed devices under the school’s authority and
        parental/school agreements. It is not directed at children as a public consumer service.
      </Typography.Paragraph>

      <Typography.Title level={4}>Contact</Typography.Title>
      <Typography.Paragraph>
        For privacy questions about your school’s use of KFilter, contact your school administration
        or use the{' '}
        <Link to="/support">support page</Link>.
      </Typography.Paragraph>

      <Typography.Paragraph>
        <Link to="/">Back to home</Link>
      </Typography.Paragraph>
    </div>
  )
}
