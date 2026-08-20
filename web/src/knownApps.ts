const knownNames: Record<string, string> = {
  'com.apple.mobilesafari': 'Safari',
  'com.apple.webapp': 'אפליקציית אינטרנט',
  'com.kfilter.portal': 'KFilter',
  'com.apple.appstore': 'App Store',
  'com.apple.mobilephone': 'טלפון',
  'com.apple.mobilemail': 'דואר',
  'com.apple.mobilesms': 'הודעות',
  'com.apple.mobilecal': 'לוח שנה',
  'com.apple.mobilenotes': 'פתקים',
  'com.apple.reminders': 'תזכורות',
  'com.apple.mobiletimer': 'שעון',
  'com.apple.calculator': 'מחשבון',
  'com.apple.camera': 'מצלמה',
  'com.apple.mobileslideshow': 'תמונות',
  'com.apple.preferences': 'הגדרות',
  'com.apple.music': 'מוזיקה',
  'com.apple.maps': 'מפות',
  'com.apple.documentsapp': 'קבצים',
  'com.apple.weather': 'מזג אוויר',
  'com.apple.passbook': 'Wallet',
  'com.apple.health': 'בריאות',
  'com.apple.fitness': 'כושר',
  'com.apple.news': 'חדשות',
  'com.apple.tv': 'TV',
  'com.apple.podcasts': 'פודקאסטים',
  'com.apple.ibooks': 'ספרים',
  'com.apple.facetime': 'FaceTime',
  'com.apple.home': 'הבית',
  'com.apple.shortcuts': 'קיצורים',
  'com.apple.translate': 'תרגום',
  'com.apple.measure': 'מדידה',
  'com.apple.compass': 'מצפן',
  'com.apple.voicememos': 'תזכירים קוליים',
  'com.apple.tips': 'טיפים',
  'com.apple.magnifier': 'זכוכית מגדלת',
  'com.apple.findmy': 'חיפוש',
  'com.apple.contacts': 'אנשי קשר',
  'com.apple.keynote': 'Keynote',
  'com.apple.numbers': 'Numbers',
  'com.apple.pages': 'Pages',
  'com.apple.testflight': 'TestFlight',
}

export function knownAppName(bundleId?: string): string {
  const key = (bundleId || '').trim().toLowerCase()
  return knownNames[key] || ''
}

export function appTitle(
  meta?: { app_name?: string; bundle_id?: string } | null,
  bundleId?: string,
): string {
  const id = (bundleId || meta?.bundle_id || '').trim()
  const name = (meta?.app_name || '').trim()
  if (name && name.toLowerCase() !== id.toLowerCase()) return name
  return knownAppName(id) || name || id || '—'
}
