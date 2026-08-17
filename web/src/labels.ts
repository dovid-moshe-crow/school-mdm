import type { Device, Group } from './api'

export function deviceLabel(d: Device | string, devices?: Device[]): string {
  const row = typeof d === 'string' ? devices?.find((x) => x.enrollment_id === d) : d
  if (!row) return typeof d === 'string' ? d : d.enrollment_id
  if (row.name && row.serial_number) return `${row.name} · ${row.serial_number}`
  return row.name || row.serial_number || row.enrollment_id
}

export function deviceMatches(d: Device, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  return [d.name, d.serial_number, d.enrollment_id].some((s) =>
    (s || '').toLowerCase().includes(q),
  )
}

export function groupMatches(g: Group, query: string): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  return [g.name, g.description, g.id].some((s) => (s || '').toLowerCase().includes(q))
}

export function deviceOptions(devices: Device[]) {
  return devices.map((d) => ({ value: d.enrollment_id, label: deviceLabel(d) }))
}

export function groupOptions(groups: Group[]) {
  return groups.map((g) => ({ value: g.id, label: g.name }))
}

export const searchableSelect = {
  showSearch: true,
  optionFilterProp: 'label' as const,
  filterOption: (input: string, option?: { label?: unknown }) =>
    String(option?.label ?? '')
      .toLowerCase()
      .includes(input.trim().toLowerCase()),
}
