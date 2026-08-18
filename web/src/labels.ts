import type { Device, Group, WhitelistPack } from './api'
import { matchesQuery, searchText } from './search'

export function deviceLabel(d: Device | string, devices?: Device[]): string {
  const row = typeof d === 'string' ? devices?.find((x) => x.enrollment_id === d) : d
  if (!row) return typeof d === 'string' ? d : d.enrollment_id
  if (row.name && row.serial_number) return `${row.name} · ${row.serial_number}`
  return row.name || row.serial_number || row.enrollment_id
}

export function deviceSearchText(d: Device): string {
  return searchText(d.name, d.serial_number, d.enrollment_id)
}

export function groupSearchText(g: Group): string {
  return searchText(g.name, g.description, g.id)
}

export function packSearchText(p: WhitelistPack): string {
  return searchText(p.name, p.description, p.id)
}

export function deviceMatches(d: Device, query: string): boolean {
  return matchesQuery(query, deviceSearchText(d))
}

export function groupMatches(g: Group, query: string): boolean {
  return matchesQuery(query, groupSearchText(g))
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
    matchesQuery(input, String(option?.label ?? '')),
}
