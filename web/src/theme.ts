import type { ThemeConfig } from 'antd'

export const theme: ThemeConfig = {
  token: {
    colorPrimary: '#0b6e4f',
    colorInfo: '#0b6e4f',
    borderRadius: 6,
    fontFamily: 'Rubik, "Segoe UI", Tahoma, Arial, sans-serif',
  },
  components: {
    Layout: {
      bodyBg: '#f5f6f8',
      headerBg: '#ffffff',
    },
    Card: {
      paddingLG: 16,
    },
  },
}
