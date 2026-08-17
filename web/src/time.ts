import dayjs from 'dayjs'
import relativeTime from 'dayjs/plugin/relativeTime'
import 'dayjs/locale/he'

dayjs.extend(relativeTime)
dayjs.locale('he')

/** Hebrew relative time, e.g. "לפני דקה", "לפני שעתיים". */
export function formatRelativeHe(value?: string | number | Date | null): string {
  if (value == null || value === '') return ''
  const d = dayjs(value)
  if (!d.isValid()) return String(value)
  return d.fromNow()
}

/** Absolute Hebrew date/time for tooltips / details. */
export function formatAbsoluteHe(value?: string | number | Date | null): string {
  if (value == null || value === '') return ''
  const d = dayjs(value)
  if (!d.isValid()) return String(value)
  return d.format('D.M.YYYY HH:mm')
}
