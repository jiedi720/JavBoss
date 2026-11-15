import { useEffect, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import ImageRoundedIcon from '@mui/icons-material/ImageRounded'
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded'
import SaveRoundedIcon from '@mui/icons-material/SaveRounded'
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded'
import { updateJavIdolCover, updateJavIdolCoverURL, uploadJavIdolCover } from '@/features/jav/api'
import { COVER_VISIBLE_RATIO, PORTRAIT_COVER_ASPECT } from '@/features/jav/coverCrop'
import AppModal from '@/shared/ui/AppModal'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'
import { getIdolDisplayName } from '@/utils/javIdol'
import { ACTOR_GENDER_MALE } from '@/utils/actorTabs'

const DEFAULT_CROP_LEFT = 1 - COVER_VISIBLE_RATIO

function normalizeCropLeft(value) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return DEFAULT_CROP_LEFT
  return Math.min(Math.max(parsed, 0), 1)
}

function getCoverVisibleRatio(size) {
  const width = Number(size?.width)
  const height = Number(size?.height)
  if (!Number.isFinite(width) || !Number.isFinite(height) || width <= 0 || height <= 0) {
    return COVER_VISIBLE_RATIO
  }
  const aspect = width / height
  if (aspect <= 0) return COVER_VISIBLE_RATIO
  return Math.min(1, PORTRAIT_COVER_ASPECT / aspect)
}

function normalizeDynamicCropLeft(value, maxCropLeft) {
  const parsed = Number(value)
  const max = Number(maxCropLeft)
  if (!Number.isFinite(parsed)) return 0
  if (!Number.isFinite(max) || max <= 0) return 0
  return Math.min(Math.max(parsed, 0), max)
}

// Male actors keep the upload / image-URL editor and, like the female idol
// editor, can pick which slice of a wide cover the portrait card shows.
export default function JavActorCoverModal({
  open,
  item,
  preferChineseName = false,
  onClose,
  onSaved,
}) {
  const fileRef = useRef(null)
  const previewRef = useRef(null)
  const [file, setFile] = useState(null)
  const [coverUrl, setCoverUrl] = useState('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [cropLeft, setCropLeft] = useState(DEFAULT_CROP_LEFT)
  const [dragging, setDragging] = useState(false)
  const [imageSize, setImageSize] = useState(null)
  const [imageFailed, setImageFailed] = useState(false)

  const actorId = Number(item?.id)
  const coverCode = String(item?.cover_image || item?.cover_code || '').trim()
  const coverSrc = coverCode ? `/jav/${encodeURIComponent(coverCode)}/cover` : ''
  const visibleRatio = getCoverVisibleRatio(imageSize)
  const maxCropLeft = Math.max(0, 1 - visibleRatio)
  const displayCropLeft = Math.min(cropLeft, maxCropLeft)

  useEffect(() => {
    if (!open) return
    setFile(null)
    setCoverUrl('')
    setError('')
    setCropLeft(normalizeCropLeft(item?.cover_crop_left ?? DEFAULT_CROP_LEFT))
    setDragging(false)
    setImageSize(null)
    setImageFailed(false)
  }, [item?.id, item?.cover_crop_left, open])

  useEffect(() => {
    setImageSize(null)
    setImageFailed(false)
  }, [coverSrc])

  if (!open || !Number.isFinite(actorId) || actorId <= 0) return null

  const setCropFromClientX = (clientX) => {
    const rect = previewRef.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0) return
    const ratio = (clientX - rect.left) / rect.width
    setCropLeft(normalizeDynamicCropLeft(ratio - visibleRatio / 2, maxCropLeft))
  }

  const handlePointerDown = (event) => {
    if (!coverSrc || imageFailed) return
    event.preventDefault()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    setDragging(true)
    setCropFromClientX(event.clientX)
  }

  const handlePointerMove = (event) => {
    if (!dragging) return
    setCropFromClientX(event.clientX)
  }

  const handlePointerUp = () => setDragging(false)

  const handleSave = async () => {
    if (saving) return
    setSaving(true)
    setError('')
    try {
      let updated = item
      const url = coverUrl.trim()
      if (file) {
        updated = await uploadJavIdolCover(actorId, file, ACTOR_GENDER_MALE)
      } else if (url) {
        updated = await updateJavIdolCoverURL(actorId, url, ACTOR_GENDER_MALE)
      }
      // jav_id = 0 keeps the automatic / uploaded cover and stores only the
      // crop position, so adjusting the position works without uploading.
      updated = await updateJavIdolCover(
        actorId,
        { javId: 0, cropLeft: displayCropLeft },
        ACTOR_GENDER_MALE
      )
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

      <div className="min-h-0 flex-1 overflow-y-auto p-4">
        <div>
          <div className="mb-2 flex items-center justify-between gap-3">
            <div className="min-w-0">
              <div className="text-[13px] font-semibold text-black dark:text-ui-text">
                {zh('显示位置', 'Visible area')}
              </div>
              {coverCode ? (
                <div className="truncate text-xs text-slate-500 dark:text-ui-faint">
                  {coverCode}
                </div>
              ) : null}
            </div>
            <div className="text-xs tabular-nums text-slate-500 dark:text-ui-faint">
              {Math.round(displayCropLeft * 100)}%
            </div>
          </div>

          <div
            ref={previewRef}
            className="relative w-full select-none overflow-hidden rounded bg-slate-100 dark:bg-ui-muted"
            onPointerDown={handlePointerDown}
            onPointerMove={handlePointerMove}
            onPointerUp={handlePointerUp}
            onPointerCancel={handlePointerUp}
          >
            {coverSrc && !imageFailed ? (
              <img
                src={coverSrc}
                alt={coverCode}
                className="block w-full"
                draggable={false}
                onLoad={(event) => {
                  const img = event.currentTarget
                  setImageSize({ width: img.naturalWidth, height: img.naturalHeight })
                }}
                onError={() => {
                  setImageFailed(true)
                  setImageSize(null)
                }}
              />
            ) : (
              <div className="flex aspect-[800/538] items-center justify-center text-slate-400">
                <ImageRoundedIcon sx={{ fontSize: 40 }} />
              </div>
            )}
            {coverSrc && !imageFailed ? (
              <div
                className="absolute inset-y-0 border-2 border-white bg-white/10 shadow-[0_0_0_999px_rgba(15,23,42,0.75)]"
                style={{
                  left: `${displayCropLeft * 100}%`,
                  width: `${visibleRatio * 100}%`,
                }}
              >
                <div className="absolute inset-y-0 left-0 w-1 bg-white/80" />
                <div className="absolute inset-y-0 right-0 w-1 bg-white/80" />
              </div>
            ) : null}
          </div>

          <div className="mt-1 text-xs text-slate-500 dark:text-ui-faint">
            {zh(
              '长方形图片可以拖动或滑动，选择竖版卡片里显示哪一块。',
              'Drag the image or use the slider to pick which part of a wide image the portrait card shows.'
            )}
          </div>

          <label className="mt-3 flex items-center gap-3 text-sm text-slate-700 dark:text-ui-secondary">
            <span className="w-10 shrink-0">{zh('起点', 'Start')}</span>
            <input
              type="range"
              min="0"
              max={maxCropLeft}
              step="0.001"
              value={displayCropLeft}
              onChange={(event) =>
                setCropLeft(normalizeDynamicCropLeft(event.target.value, maxCropLeft))
              }
              className="min-w-0 flex-1 accent-slate-900 dark:accent-blue-400"
            />
          </label>
        </div>

        <div className="mt-5 space-y-5 border-t border-slate-200 pt-4 dark:border-ui-border">
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

      <div className="flex items-center justify-between gap-2 border-t border-slate-200 px-4 py-3 dark:border-ui-border">
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
          onClick={() => {
            setCropLeft(DEFAULT_CROP_LEFT)
            setImageFailed(false)
          }}
          disabled={saving}
        >
          <RestartAltRoundedIcon sx={{ fontSize: 17 }} />
          {zh('重置位置', 'Reset position')}
        </button>
        <div className="flex items-center gap-2">
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
      </div>
    </AppModal>
  )
}
