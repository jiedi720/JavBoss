import { useEffect, useState } from 'react'
import { Switch } from '@mui/material'
import { useAppTheme } from '@/app/theme/AppThemeProvider'
import {
  applyCoverBlur,
  COVER_BLUR_STORAGE_KEY,
  readCoverBlur,
  setCoverBlur,
} from '@/app/theme/coverBlur'
import { zh } from '@/utils/i18n'

export default function AppearanceSettings() {
  const { preference, setPreference } = useAppTheme()
  const [storageError, setStorageError] = useState(false)
  const [coverBlur, setCoverBlurEnabled] = useState(readCoverBlur)

  useEffect(() => {
    applyCoverBlur(coverBlur)
  }, [coverBlur])

  useEffect(() => {
    const onStorage = (event) => {
      if (event.key === COVER_BLUR_STORAGE_KEY || event.key === null) {
        setCoverBlurEnabled(readCoverBlur())
      }
    }
    window.addEventListener('storage', onStorage)
    return () => window.removeEventListener('storage', onStorage)
  }, [])

  const handleCoverBlurChange = (enabled) => {
    setCoverBlurEnabled(enabled)
    if (!setCoverBlur(enabled)) setStorageError(true)
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-ui-border dark:bg-ui-surface">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <label
            htmlFor="appearance-theme"
            className="text-sm font-semibold text-zinc-800 dark:text-ui-text"
          >
            {zh('外观主题', 'Appearance theme')}
          </label>
        </div>
        <select
          id="appearance-theme"
          value={preference}
          disabled={!setPreference}
          onChange={(event) => setStorageError(!setPreference(event.target.value))}
          className="form-field rounded-xl border border-zinc-200 bg-white px-3 py-1.5 text-sm text-zinc-800 focus-visible:border-blue-500 focus-visible:ring-2 focus-visible:ring-blue-100"
        >
          <option value="light">{zh('浅色', 'Light')}</option>
          <option value="dark">{zh('深色', 'Dark')}</option>
          <option value="system">{zh('跟随系统', 'System')}</option>
        </select>
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-zinc-100 pt-4 dark:border-ui-border">
        <div className="min-w-0">
          <span className="text-sm font-semibold text-zinc-800 dark:text-ui-text">
            {zh('封面模糊', 'Blur covers')}
          </span>
          <p className="mt-1 text-xs leading-5 text-zinc-500 dark:text-ui-secondary">
            {zh(
              '开启后所有封面以毛玻璃效果模糊显示',
              'Show every cover behind a frosted-glass blur'
            )}
          </p>
        </div>
        <Switch
          checked={coverBlur}
          onChange={(event) => handleCoverBlurChange(event.target.checked)}
          inputProps={{ 'aria-label': zh('封面模糊', 'Blur covers') }}
          size="small"
        />
      </div>
      {storageError && (
        <p role="alert" className="mt-3 text-xs text-amber-700 dark:text-amber-300">
          {zh(
            '浏览器无法保存设置，本次选择仅在当前页面生效。',
            'The browser could not save your settings. This choice only applies to the current page.'
          )}
        </p>
      )}
    </section>
  )
}
