export const COVER_BLUR_STORAGE_KEY = 'jb.coverBlur'
export const COVER_BLUR_CLASS = 'jb-cover-blur'

export function readCoverBlur() {
  try {
    return localStorage.getItem(COVER_BLUR_STORAGE_KEY) === 'true'
  } catch {
    return false
  }
}

export function applyCoverBlur(enabled) {
  if (typeof document === 'undefined') return
  document.documentElement.classList.toggle(COVER_BLUR_CLASS, Boolean(enabled))
}

export function setCoverBlur(enabled) {
  const next = Boolean(enabled)
  applyCoverBlur(next)
  try {
    localStorage.setItem(COVER_BLUR_STORAGE_KEY, String(next))
    return true
  } catch {
    return false
  }
}
