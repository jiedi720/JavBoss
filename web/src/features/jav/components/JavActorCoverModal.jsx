import { useEffect, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import ImageRoundedIcon from '@mui/icons-material/ImageRounded'
import SaveRoundedIcon from '@mui/icons-material/SaveRounded'
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded'
import { updateJavIdolCoverURL, uploadJavIdolCover } from '@/features/jav/api'
import AppModal from '@/shared/ui/AppModal'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'
import { getIdolDisplayName } from '@/utils/javIdol'
import { ACTOR_GENDER_MALE } from '@/utils/actorTabs'

// Male actors get their own compact cover editor: upload a local image or paste
// an image URL (same behaviour as the "Cover URL" field in the JAV edit modal).
// The female idol cover editor keeps its work-based cover picker and crop tool.
export default function JavActorCoverModal({
  open,
  item,
  preferChineseName = false,
  onClose,
  onSaved,
}) {
  const fileRef = useRef(null)
  const [file, setFile] = useState(null)
  const [coverUrl, setCoverUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')

  const actorId = Number(item?.id)
  const coverCode = String(item?.cover_image || item?.cover_code || '').trim()
  const coverSrc = coverCode ? `/jav/${encodeURIComponent(coverCode)}/cover` : ''

  useEffect(() => {
    if (!open) return
    setFile(null)
    setCoverUrl('')
    setError('')
  }, [item?.id, open])

  if (!open || !Number.isFinite(actorId) || actorId <= 0) return null

  const handleSave = async () => {
    if (saving) return
    const url = coverUrl.trim()
    if (!file && !url) {
      setError(
        zh('请选择要上传的图片，或输入封面图片链接。', 'Pick an image or enter a cover image URL.')
      )
      return
    }
    setSaving(true)
    setError('')
    try {
      const updated = file
        ? await uploadJavIdolCover(actorId, file, ACTOR_GENDER_MALE)
        : await updateJavIdolCoverURL(actorId, url, ACTOR_GENDER_MALE)
      onSaved?.(updated)
      onClose?.()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <AppModal
      ariaLabel={zh('编辑男优封面', 'Edit actor cover')}
      className="px-4 py-6"
      closeDisabled={saving}
      contentClassName="flex max-h-[92vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white dark:bg-ui-surface shadow-2xl"
      onClose={onClose}
    >
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-ui-border">
        <div className="min-w-0">
          <div className="truncate text-base font-semibold text-slate-950 dark:text-ui-text">
            {zh('编辑男优封面', 'Edit actor cover')}
          </div>
          <div className="truncate text-xs text-slate-500 dark:text-ui-faint">
            {getIdolDisplayName(item, preferChineseName)}
          </div>
        </div>
        <button
          type="button"
          className="flex h-8 w-8 items-center justify-center rounded-full text-slate-500 hover:bg-slate-100 dark:text-ui-faint dark:hover:bg-ui-hover"
          onClick={onClose}
          aria-label={zh('关闭', 'Close')}
        >
          <CloseOutlinedIcon sx={{ fontSize: 18 }} />
        </button>
      </div>

      <div className="grid min-h-0 flex-1 gap-0 overflow-y-auto md:grid-cols-[12rem_minmax(0,1fr)]">
        <div className="border-b border-slate-200 p-3 dark:border-ui-border md:border-b-0 md:border-r">
          <div className="mb-2 text-[13px] font-semibold text-black dark:text-ui-text">
            {zh('当前封面', 'Current cover')}
          </div>
          <div className="overflow-hidden rounded bg-slate-100 dark:bg-ui-muted">
            {coverSrc ? (
              <img src={coverSrc} alt={coverCode} className="block w-full" />
            ) : (
              <div className="flex aspect-[2/3] items-center justify-center text-slate-400">
                <ImageRoundedIcon sx={{ fontSize: 40 }} />
              </div>
            )}
          </div>
          {coverCode ? (
            <div className="mt-2 break-all text-xs text-slate-500 dark:text-ui-faint">
              {coverCode}
            </div>
          ) : null}
        </div>

        <div className="space-y-5 p-4">
          <div>
            <div className="text-[13px] font-semibold text-black dark:text-ui-text">
              {zh('上传图片', 'Upload image')}
            </div>
            <input
              ref={fileRef}
              type="file"
              accept="image/jpeg,image/png"
              className="hidden"
              onChange={(event) => {
                setFile(event.target.files?.[0] || null)
                setError('')
              }}
              disabled={saving}
            />
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <button
                type="button"
                className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
                onClick={() => fileRef.current?.click()}
                disabled={saving}
              >
                <UploadFileRoundedIcon sx={{ fontSize: 17 }} />
                {zh('选择本地图片', 'Choose image')}
              </button>
              <span className="min-w-0 truncate text-xs text-slate-500 dark:text-ui-faint">
                {file ? file.name : zh('JPG / PNG，小于 12MB', 'JPG / PNG, under 12MB')}
              </span>
            </div>
          </div>

          <div className="border-t border-slate-200 pt-4 dark:border-ui-border">
            <label
              className="block text-[13px] font-semibold text-black dark:text-ui-text"
              htmlFor="actor-cover-url"
            >
              {zh('封面链接', 'Cover URL')}
            </label>
            <input
              id="actor-cover-url"
              type="url"
              value={coverUrl}
              onChange={(event) => {
                setCoverUrl(event.target.value)
                if (error) setError('')
              }}
              placeholder="https://..."
              className="form-field mt-2 w-full rounded-md border border-gray-300 px-3 py-2 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
              disabled={saving}
            />
            <div className="mt-1 text-xs text-gray-500 dark:text-ui-faint">
              {zh(
                '当封面缺失或显示错误时，可手动输入封面图片链接；保存后会自动下载到本地并完成更新。',
                'If the cover is missing or incorrect, enter an image URL; saving downloads it locally and updates the cover.'
              )}
            </div>
          </div>

          {file && coverUrl.trim() ? (
            <div className="text-xs text-amber-600 dark:text-amber-300">
              {zh(
                '已同时选择图片和链接，保存时将优先使用上传的图片。',
                'Both an image and a URL were provided; the uploaded image takes priority.'
              )}
            </div>
          ) : null}

          {error ? <div className="text-sm text-rose-600 dark:text-rose-300">{error}</div> : null}
        </div>
      </div>

      <div className="flex items-center justify-end gap-2 border-t border-slate-200 px-4 py-3 dark:border-ui-border">
        <button
          type="button"
          className="rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
          onClick={onClose}
          disabled={saving}
        >
          {zh('取消', 'Cancel')}
        </button>
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded bg-ui-primary px-3 py-1.5 text-sm text-white enabled:hover:bg-ui-primary-hover disabled:cursor-not-allowed disabled:opacity-50"
          onClick={handleSave}
          disabled={saving}
        >
          <SaveRoundedIcon sx={{ fontSize: 17 }} />
          {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
        </button>
      </div>
    </AppModal>
  )
}
