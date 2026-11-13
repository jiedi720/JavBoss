import { preferChineseTitle } from '@/features/jav/titleTranslation'
import { useEffect, useMemo, useRef, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import ImageRoundedIcon from '@mui/icons-material/ImageRounded'
import RestartAltRoundedIcon from '@mui/icons-material/RestartAltRounded'
import SaveRoundedIcon from '@mui/icons-material/SaveRounded'
import UploadFileRoundedIcon from '@mui/icons-material/UploadFileRounded'
import {
  fetchJavIdolCoverOptions,
  updateJavIdolCover,
  updateJavIdolCoverURL,
  uploadJavIdolCover,
} from '@/features/jav/api'
import AppModal from '@/shared/ui/AppModal'
import { getJavDisplayTitle } from '@/utils/jav'
import { useStore } from '@/store'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'
import { getIdolDisplayName } from '@/utils/javIdol'
import { actorGenderOf, actorTerms } from '@/utils/actorTabs'
import { COVER_VISIBLE_RATIO, PORTRAIT_COVER_ASPECT } from '@/features/jav/coverCrop'

export const IDOL_COVER_VISIBLE_RATIO = COVER_VISIBLE_RATIO
export const IDOL_COVER_DEFAULT_CROP_LEFT = 1 - IDOL_COVER_VISIBLE_RATIO

export function normalizeIdolCoverCropLeft(value) {
  const parsed = Number(value)
  if (!Number.isFinite(parsed)) return IDOL_COVER_DEFAULT_CROP_LEFT
  return Math.min(Math.max(parsed, 0), 1)
}

function getCoverVisibleRatio(size) {
  const imageWidth = Number(size?.width)
  const imageHeight = Number(size?.height)
  if (
    !Number.isFinite(imageWidth) ||
    !Number.isFinite(imageHeight) ||
    imageWidth <= 0 ||
    imageHeight <= 0
  ) {
    return IDOL_COVER_VISIBLE_RATIO
  }
  const imageAspect = imageWidth / imageHeight
  if (imageAspect <= 0) return IDOL_COVER_VISIBLE_RATIO
  return Math.min(1, PORTRAIT_COVER_ASPECT / imageAspect)
}

function normalizeDynamicCropLeft(value, maxCropLeft) {
  const parsed = Number(value)
  const max = Number(maxCropLeft)
  if (!Number.isFinite(parsed)) return 0
  if (!Number.isFinite(max) || max <= 0) return 0
  return Math.min(Math.max(parsed, 0), max)
}

function getIdolCoverCode(item) {
  return String(item?.cover_code || '').trim()
}

export default function JavIdolCoverModal({
  open,
  gender,
  item,
  preferChineseName = false,
  onClose,
  onSaved,
}) {
  const previewRef = useRef(null)
  const coverFileRef = useRef(null)
  const [options, setOptions] = useState([])
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [selectedJavId, setSelectedJavId] = useState(0)
  const [cropLeft, setCropLeft] = useState(IDOL_COVER_DEFAULT_CROP_LEFT)
  const [dragging, setDragging] = useState(false)
  const [imageFailed, setImageFailed] = useState(false)
  const [imageSize, setImageSize] = useState(null)
  // A cover uploaded or downloaded from a URL bypasses the work-based cover
  // list, so its code is tracked locally until the parent item refreshes.
  const [customCoverCode, setCustomCoverCode] = useState('')
  const [coverURLInput, setCoverURLInput] = useState('')
  const [coverBusy, setCoverBusy] = useState(false)
  const [coverVersion, setCoverVersion] = useState(0)

  const idolId = Number(item?.id)
  const itemCoverCode = getIdolCoverCode(item)
  const actorGender = actorGenderOf(gender, item)
  const terms = actorTerms(actorGender)

  useEffect(() => {
    if (!open || !Number.isFinite(idolId) || idolId <= 0) return undefined
    let cancelled = false
    setLoading(true)
    setError('')
    setOptions([])
    setSelectedJavId(Number(item?.cover_jav_id) || 0)
    setCropLeft(normalizeIdolCoverCropLeft(item?.cover_crop_left ?? IDOL_COVER_DEFAULT_CROP_LEFT))
    fetchJavIdolCoverOptions(idolId, actorGender)
      .then((items) => {
        if (cancelled) return
        setOptions(items)
        setSelectedJavId((current) => {
          if (current && items.some((option) => Number(option.id) === current)) return current
          const matched = items.find(
            (option) => String(option?.code || '').trim() === itemCoverCode
          )
          return matched ? Number(matched.id) : 0
        })
      })
      .catch((err) => {
        if (!cancelled) setError(getErrorMessage(err))
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [actorGender, idolId, itemCoverCode, item?.cover_crop_left, item?.cover_jav_id, open])

  useEffect(() => {
    if (!open) {
      setDragging(false)
      setImageFailed(false)
      setImageSize(null)
      setCoverURLInput('')
      return
    }
    setCustomCoverCode(String(item?.cover_image || '').trim())
    setCoverURLInput('')
  }, [item?.cover_image, open])

  const selectedOption = useMemo(
    () => options.find((option) => Number(option?.id) === Number(selectedJavId)) || null,
    [options, selectedJavId]
  )
  const previewCode = String(customCoverCode || selectedOption?.code || itemCoverCode).trim()
  const coverSrc = previewCode
    ? `/jav/${encodeURIComponent(previewCode)}/cover${coverVersion ? `?v=${coverVersion}` : ''}`
    : ''
  const translateTitle = useStore((state) => preferChineseTitle(state.config))
  const title = selectedOption ? getJavDisplayTitle(selectedOption, translateTitle) : ''
  const visibleRatio = getCoverVisibleRatio(imageSize)
  const maxCropLeft = Math.max(0, 1 - visibleRatio)
  const displayCropLeft = Math.min(cropLeft, maxCropLeft)

  useEffect(() => {
    setImageSize(null)
  }, [coverSrc])

  const setCropFromClientX = (clientX) => {
    const rect = previewRef.current?.getBoundingClientRect()
    if (!rect || rect.width <= 0) return
    const ratio = (clientX - rect.left) / rect.width
    setCropLeft(normalizeDynamicCropLeft(ratio - visibleRatio / 2, maxCropLeft))
  }

  const handlePointerDown = (event) => {
    if (!previewCode) return
    event.preventDefault()
    event.currentTarget.setPointerCapture?.(event.pointerId)
    setDragging(true)
    setCropFromClientX(event.clientX)
  }

  const handlePointerMove = (event) => {
    if (!dragging) return
    setCropFromClientX(event.clientX)
  }

  const handlePointerUp = () => {
    setDragging(false)
  }

  const handleSave = async () => {
    if (!Number.isFinite(idolId) || idolId <= 0 || saving) return
    setSaving(true)
    setError('')
    try {
      const updated = await updateJavIdolCover(
        idolId,
        {
          javId: Number(selectedJavId) || 0,
          cropLeft: displayCropLeft,
        },
        actorGender
      )
      onSaved?.(updated)
      onClose?.()
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  const applyCoverResult = (updated) => {
    const nextCode = String(updated?.cover_code || '').trim()
    setSelectedJavId(0)
    setCustomCoverCode(nextCode)
    setImageFailed(false)
    setImageSize(null)
    setCoverVersion((current) => current + 1)
    onSaved?.(updated)
  }

  const handleCoverUpload = async (event) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file || !Number.isFinite(idolId) || idolId <= 0 || coverBusy) return
    setCoverBusy(true)
    setError('')
    try {
      const updated = await uploadJavIdolCover(idolId, file, actorGender)
      applyCoverResult(updated)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setCoverBusy(false)
    }
  }

  const handleCoverURLApply = async () => {
    const url = coverURLInput.trim()
    if (!url || !Number.isFinite(idolId) || idolId <= 0 || coverBusy) return
    setCoverBusy(true)
    setError('')
    try {
      const updated = await updateJavIdolCoverURL(idolId, url, actorGender)
      setCoverURLInput('')
      applyCoverResult(updated)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setCoverBusy(false)
    }
  }

  if (!open) return null

  return (
    <AppModal
      ariaLabel={zh(`${terms.zhNoun}封面`, `${terms.enSingularCap} cover`)}
      className="px-4 py-6"
      closeDisabled={saving}
      contentClassName="flex max-h-[92vh] w-full max-w-5xl flex-col overflow-hidden rounded-lg bg-white dark:bg-ui-surface shadow-2xl"
      onClose={onClose}
    >
      <div className="flex items-center justify-between border-b border-slate-200 px-4 py-3 dark:border-ui-border">
        <div className="min-w-0">
          <div className="truncate text-base font-semibold text-slate-950 dark:text-ui-text">
            {getIdolDisplayName(item, preferChineseName)}
          </div>
          <div className="text-xs text-slate-500 dark:text-ui-faint">{zh('封面', 'Cover')}</div>
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

      <div className="grid min-h-0 flex-1 gap-0 overflow-hidden md:grid-cols-[18rem_minmax(0,1fr)]">
        <div className="min-h-[12rem] overflow-y-auto border-b border-slate-200 dark:border-ui-border md:border-b-0 md:border-r">
          <button
            type="button"
            className={`flex w-full items-center gap-2 border-b px-3 py-2 text-left text-sm ${
              selectedJavId === 0
                ? 'bg-slate-900 text-white dark:bg-ui-blue-surface dark:text-blue-300'
                : 'hover:bg-slate-50 dark:hover:bg-ui-hover'
            }`}
            onClick={() => {
              setSelectedJavId(0)
              setImageFailed(false)
            }}
          >
            <ImageRoundedIcon sx={{ fontSize: 17 }} />
            <span className="min-w-0 flex-1 truncate">{zh('自动选择', 'Auto')}</span>
          </button>
          {loading ? (
            <div className="px-3 py-4 text-sm text-slate-500 dark:text-ui-faint">
              {zh('加载中…', 'Loading...')}
            </div>
          ) : options.length > 0 ? (
            options.map((option) => {
              const active = Number(option.id) === Number(selectedJavId)
              const optionTitle = getJavDisplayTitle(option, translateTitle)
              return (
                <button
                  key={option.id}
                  type="button"
                  className={`flex w-full flex-col gap-0.5 border-b px-3 py-2 text-left text-sm ${
                    active
                      ? 'bg-slate-900 text-white dark:bg-ui-blue-surface dark:text-blue-300'
                      : 'hover:bg-slate-50 dark:hover:bg-ui-hover'
                  }`}
                  onClick={() => {
                    setSelectedJavId(Number(option.id))
                    setImageFailed(false)
                  }}
                >
                  <span className="flex w-full items-center gap-2">
                    <span className="font-semibold">{option.code}</span>
                    {option.solo ? (
                      <span
                        className={`rounded px-1.5 py-0.5 text-[10px] ${
                          active
                            ? 'bg-white/20 text-white'
                            : 'bg-emerald-50 text-emerald-700 dark:bg-emerald-950 dark:text-emerald-300'
                        }`}
                      >
                        {zh('单人', 'Solo')}
                      </span>
                    ) : null}
                  </span>
                  {optionTitle && optionTitle !== option.code ? (
                    <span className="line-clamp-2 text-xs opacity-75">{optionTitle}</span>
                  ) : null}
                </button>
              )
            })
          ) : (
            <div className="px-3 py-4 text-sm text-slate-500 dark:text-ui-faint">
              {zh('暂无可用作品', 'No available works')}
            </div>
          )}
        </div>

        <div className="min-h-0 overflow-y-auto p-4">
          <div className="mx-auto max-w-3xl">
            <div className="mb-2 flex items-center justify-between gap-3">
              <div className="min-w-0">
                <div className="truncate text-sm font-semibold text-slate-900 dark:text-ui-text">
                  {previewCode || zh('无封面', 'No cover')}
                </div>
                {title && title !== previewCode ? (
                  <div className="truncate text-xs text-slate-500 dark:text-ui-faint">{title}</div>
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
                  alt={previewCode}
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
                <div className="flex aspect-[800/538] items-center justify-center text-sm text-slate-500 dark:text-ui-faint">
                  {zh('封面待下载', 'Cover pending')}
                </div>
              )}
              {previewCode && !imageFailed ? (
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

            <label className="mt-4 flex items-center gap-3 text-sm text-slate-700 dark:text-ui-secondary">
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

            <div className="mt-5 rounded-md border border-slate-200 p-3 dark:border-ui-border">
              <div className="text-[13px] font-semibold text-black dark:text-ui-text">
                {zh('自定义封面', 'Custom cover')}
              </div>
              <p className="mt-1 text-xs text-slate-500 dark:text-ui-faint">
                {zh(
                  '当封面缺失或显示错误时，可上传本地图片，或输入封面图片链接；上传或应用后会立即下载到本地并完成更新。',
                  'If the cover is missing or incorrect, upload an image or paste an image URL; it is written to the local cover directory right away.'
                )}
              </p>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <input
                  ref={coverFileRef}
                  type="file"
                  accept="image/jpeg,image/png"
                  className="hidden"
                  onChange={handleCoverUpload}
                  disabled={coverBusy}
                />
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
                  onClick={() => coverFileRef.current?.click()}
                  disabled={coverBusy || loading}
                >
                  <UploadFileRoundedIcon sx={{ fontSize: 17 }} />
                  {zh('上传图片', 'Upload image')}
                </button>
                <span className="text-xs text-slate-400 dark:text-ui-faint">
                  {zh('JPG / PNG，小于 12MB', 'JPG / PNG, under 12MB')}
                </span>
              </div>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <input
                  type="url"
                  value={coverURLInput}
                  onChange={(event) => {
                    setCoverURLInput(event.target.value)
                    if (error) setError('')
                  }}
                  placeholder="https://..."
                  className="form-field min-w-0 flex-1 rounded-md border border-gray-300 px-3 py-1.5 text-sm text-gray-900 outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-100"
                  disabled={coverBusy}
                />
                <button
                  type="button"
                  className="inline-flex items-center gap-1.5 rounded bg-ui-primary px-3 py-1.5 text-sm text-white enabled:hover:bg-ui-primary-hover disabled:cursor-not-allowed disabled:opacity-50"
                  onClick={handleCoverURLApply}
                  disabled={coverBusy || !coverURLInput.trim()}
                >
                  {coverBusy ? zh('处理中…', 'Working...') : zh('应用链接', 'Apply URL')}
                </button>
              </div>
            </div>

            {error ? (
              <div className="mt-3 text-sm text-rose-600 dark:text-rose-300">{error}</div>
            ) : null}
          </div>
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2 border-t border-slate-200 px-4 py-3 dark:border-ui-border">
        <button
          type="button"
          className="inline-flex items-center gap-1.5 rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
          onClick={() => {
            setSelectedJavId(0)
            setCropLeft(IDOL_COVER_DEFAULT_CROP_LEFT)
            setImageFailed(false)
          }}
        >
          <RestartAltRoundedIcon sx={{ fontSize: 17 }} />
          {zh('重置', 'Reset')}
        </button>
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="rounded border border-slate-300 px-3 py-1.5 text-sm text-slate-700 hover:bg-slate-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
            onClick={onClose}
          >
            {zh('取消', 'Cancel')}
          </button>
          <button
            type="button"
            className="inline-flex items-center gap-1.5 rounded bg-ui-primary px-3 py-1.5 text-sm text-white enabled:hover:bg-ui-primary-hover disabled:cursor-not-allowed disabled:opacity-50"
            disabled={saving || loading}
            onClick={handleSave}
          >
            <SaveRoundedIcon sx={{ fontSize: 17 }} />
            {saving ? zh('保存中…', 'Saving...') : zh('保存', 'Save')}
          </button>
        </div>
      </div>
    </AppModal>
  )
}
