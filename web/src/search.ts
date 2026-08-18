/** Tokenized substring match across any list fields (Hebrew/English). */
export function matchesQuery(
  query: string,
  ...parts: Array<string | number | null | undefined>
): boolean {
  const q = query.trim().toLowerCase()
  if (!q) return true
  const hay = parts.map((p) => String(p ?? '').toLowerCase()).join(' ')
  return q.split(/\s+/).every((token) => hay.includes(token))
}

export function searchText(
  ...parts: Array<string | number | null | undefined>
): string {
  return parts.filter((p) => p != null && String(p).trim() !== '').join(' ')
}
