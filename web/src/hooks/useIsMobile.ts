import { Grid } from 'antd'

/** True below Ant Design `md` (768px). */
export function useIsMobile() {
  const screens = Grid.useBreakpoint()
  // During SSR/first paint screens may be empty — treat as mobile to avoid desktop flash on phones.
  if (screens.md === undefined && screens.xs === undefined) return true
  return !screens.md
}
