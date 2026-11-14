import Tooltip from '@/shared/ui/Tooltip'
import { useEffect, useState } from 'react'
import { Popover } from '@mui/material'
import DeleteOutlineIcon from '@mui/icons-material/DeleteOutline'
import DriveFileRenameOutlineIcon from '@mui/icons-material/DriveFileRenameOutline'
import LocalOfferOutlinedIcon from '@mui/icons-material/LocalOfferOutlined'
import PlayArrowIcon from '@mui/icons-material/PlayArrow'
import FolderOpenIcon from '@mui/icons-material/FolderOpen'
import ManageSearchIcon from '@mui/icons-material/ManageSearch'
import { revealVideoLocation, updateVideoFavoriteRating } from '@/features/video/api'
import { useStore } from '@/store'
import { buildVideoQueryLink } from '@/navigation/queryLinks'
import { displayHostPath, hostPathsEnabled } from '@/utils/hostPath'
import {
  buildVideoFullPath,
  formatBytes,
  getVideoDisplayName,
  parseVideoFingerprint,
} from '@/utils/display'
import { zh } from '@/utils/i18n'
import PhotoLibraryOutlinedIcon from '@mui/icons-material/PhotoLibraryOutlined'
import { MovieEdit } from '@mui/icons-material'
import WatchTimeIcons from '@/features/playback/components/WatchTimeIcons'
import VideoThumbnail from '@/features/video/components/VideoThumbnail'
import { Rating } from '@mui/material'
import StarBorderRoundedIcon from '@mui/icons-material/StarBorderRounded'
import StarRoundedIcon from '@mui/icons-material/StarRounded'
import FavoriteRoundedIcon from '@mui/icons-material/FavoriteRounded'
import FavoriteBorderRoundedIcon from '@mui/icons-material/FavoriteBorderRounded'
import RemoveCircleOutlineRoundedIcon from '@mui/icons-material/RemoveCircleOutlineRounded'
import CheckBoxRoundedIcon from '@mui/icons-material/CheckBoxRounded'
import CheckBoxOutlineBlankRoundedIcon from '@mui/icons-material/CheckBoxOutlineBlankRounded'
import { getErrorMessage } from '@/utils/errors'
import JavFavoriteModal from '@/features/favorites/components/JavFavoriteModal'
import {
  createJavFavoriteGroup,
  fetchJavFavoriteGroups,
  fetchJavFavoriteSelection,
  replaceJavFavoriteGroups,
} from '@/features/favorites/api'

const coverButtonClassName =
  'flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-black/70 text-white shadow-lg shadow-black/60 hover:bg-black/85 disabled:cursor-not-allowed disabled:opacity-40'

export default function VideoCard({
  video,
  checked,
  onToggle,
  showSelection = true,
  onPlay,
  onOpenFile,
  onRevealFile,
  openFileLabel,
  onOpenTagPicker,
  showTagEditor = true,
  onOpenScreenshots,
  onOpenScrapeSettings,
  onRenameVideo,
  onDeleteVideo,
  onTagClick,
}) {
  const useHostPaths = useStore((state) => hostPathsEnabled(state.config))
  const itemFavoriteRating = Number(video?.favorite_rating) || 0
  const [favoriteRating, setFavoriteRating] = useState(itemFavoriteRating)
  const [favoriteRatingSaving, setFavoriteRatingSaving] = useState(false)
  const [favoriteRatingError, setFavoriteRatingError] = useState('')
  const [favoriteRatingEditing, setFavoriteRatingEditing] = useState(false)
  const [favoriteRatingPreview, setFavoriteRatingPreview] = useState(null)
  const favoriteRatingIconSize = 21
  const favoriteRatingTooltipValue = favoriteRatingPreview ?? favoriteRating
  const hasFavoriteRatingTooltipValue = favoriteRatingPreview !== null || favoriteRating > 0
  const favoriteRatingWidth = !favoriteRatingEditing
    ? Math.max(favoriteRating > 0 ? 1 : 0, 1) * 21
    : 5 * 21
  const [favoriteCount, setFavoriteCount] = useState(Number(video?.favorite_count) || 0)
  const [favoritesOpen, setFavoritesOpen] = useState(false)
  const [favoriteGroups, setFavoriteGroups] = useState([])
  const [favoriteSelectedIds, setFavoriteSelectedIds] = useState([])
  const [favoriteLoading, setFavoriteLoading] = useState(false)
  const [favoriteSaving, setFavoriteSaving] = useState(false)
  const [favoriteError, setFavoriteError] = useState('')

  useEffect(() => {
    setFavoriteRating(itemFavoriteRating)
  }, [video?.id, itemFavoriteRating])

  // Mirrors the JAV card: hearts in the cover's top-left, half-star precision,
  // save immediately on change and roll back when the request fails.
  const handleFavoriteRatingChange = async (event, value) => {
    event?.stopPropagation()
    const videoID = Number(video?.id)
    const numericValue = value == null ? 0 : Number(value)
    const nextRating = Math.round(numericValue * 2) / 2
    if (
      favoriteRatingSaving ||
      !Number.isFinite(videoID) ||
      videoID <= 0 ||
      !Number.isFinite(nextRating) ||
      nextRating < 0 ||
      nextRating > 5
    ) {
      return
    }
    const previousRating = favoriteRating
    setFavoriteRating(nextRating)
    setFavoriteRatingSaving(true)
    setFavoriteRatingError('')
    try {
      const updated = await updateVideoFavoriteRating(videoID, nextRating)
      const savedRating = Number(updated?.favorite_rating)
      setFavoriteRating(Number.isFinite(savedRating) ? savedRating : nextRating)
    } catch (error) {
      setFavoriteRating(previousRating)
      setFavoriteRatingError(getErrorMessage(error))
    } finally {
      setFavoriteRatingSaving(false)
    }
  }

  const handleOpenFavorites = async (event) => {
    event?.stopPropagation?.()
    setFavoritesOpen(true)
    setFavoriteLoading(true)
    setFavoriteError('')
    try {
      const [groups, ids] = await Promise.all([
        fetchJavFavoriteGroups('video'),
        fetchJavFavoriteSelection('video', video.id),
      ])
      setFavoriteGroups(Array.isArray(groups) ? groups : [])
      setFavoriteSelectedIds(Array.isArray(ids) ? ids : [])
    } catch (err) {
      setFavoriteError(String(err?.message || err))
    } finally {
      setFavoriteLoading(false)
    }
  }

  const handleSaveFavorites = async (groupIds) => {
    setFavoriteSaving(true)
    setFavoriteError('')
    try {
      await replaceJavFavoriteGroups('video', video.id, groupIds)
      setFavoriteSelectedIds(Array.isArray(groupIds) ? groupIds : [])
      setFavoriteCount(Array.isArray(groupIds) ? groupIds.length : 0)
      setFavoritesOpen(false)
    } catch (err) {
      setFavoriteError(String(err?.message || err))
    } finally {
      setFavoriteSaving(false)
    }
  }
  const [editAnchorEl, setEditAnchorEl] = useState(null)
  const displayName = getVideoDisplayName(video)
  const durationSec = Number(video?.duration_sec)
  const durationMinutes =
    Number.isFinite(durationSec) && durationSec > 0
      ? Math.max(1, Math.round(durationSec / 60))
      : null
  const meta = parseVideoFingerprint(video?.fingerprint)
  const resolution =
    meta.width && meta.height && meta.width > 0 && meta.height > 0
      ? `${meta.width}x${meta.height}`
      : ''
  const sizeText = formatBytes(meta.size || video?.size)
  const directoryPath = video?.directory?.path || video?.directory_path || ''
  const directoryId = video?.directory_id || video?.directory?.id
  const videoPath = video?.path || ''
  const relativeDirectoryPath = videoPath.split(/[/\\]/).slice(0, -1).join('/') || '.'
  const directoryHref = useStore((state) =>
    buildVideoQueryLink(
      state,
      {
        directoryId,
        subdirectory: relativeDirectoryPath,
        page: 1,
        search: '',
        tagIds: [],
        random: false,
        tempSort: '',
      },
      window.location.pathname
    )
  )
  const fullPath = buildVideoFullPath(video)
  const displayPath = displayHostPath(fullPath, useHostPaths)
  const canOpen = Boolean(directoryPath && videoPath)
  const javCode = String(video?.jav?.code || video?.locations?.[0]?.jav?.code || '').trim()
  const hasScrapeOverride = Boolean(String(video?.jav_scrape_override || '').trim())
  const canOpenFile = Boolean(onOpenFile)
  const canRevealFile = Boolean(onRevealFile)
  const thumbnailVersion = encodeURIComponent(
    [video?.cover_screenshot_name || '', video?.updated_at || ''].join('|')
  )
  const thumbnailSrc = `/videos/${video.id}/thumbnail${thumbnailVersion ? `?v=${thumbnailVersion}` : ''}`

  const handleDirectoryClick = (event) => {
    event.stopPropagation()
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0)
      return
    event.preventDefault()
    const state = useStore.getState()
    if (state.viewMode !== 'video') state.setViewMode('video')
    state.setVideoDirectoryFilter({ directory_id: directoryId, path: relativeDirectoryPath })
  }

  const handleOpenFile = async (event) => {
    event.stopPropagation()
    if (!canOpen) return
    try {
      await onOpenFile?.(video)
    } catch (err) {
      console.error(zh('打开文件失败', 'Open file failed'), err)
    }
  }

  const handleRevealFile = async (event) => {
    event.stopPropagation()
    if (!canOpen) return
    try {
      if (onRevealFile) {
        await onRevealFile(video)
      } else {
        await revealVideoLocation({ path: videoPath, dirPath: directoryPath })
      }
    } catch (err) {
      console.error(zh('打开所在位置失败', 'Reveal file failed'), err)
    }
  }

  const handleOpenScreenshots = (event) => {
    event.stopPropagation()
    onOpenScreenshots?.(video)
  }

  const handleOpenScrapeSettings = (event) => {
    event.stopPropagation()
    onOpenScrapeSettings?.(video)
  }

  const openEditMenu = (event) => {
    event.stopPropagation()
    setEditAnchorEl(event.currentTarget)
  }

  const closeEditMenu = () => {
    setEditAnchorEl(null)
  }

  const handleRename = (event) => {
    event.stopPropagation()
    closeEditMenu()
    onRenameVideo?.(video)
  }

  const handleDelete = (event) => {
    event.stopPropagation()
    closeEditMenu()
    onDeleteVideo?.(video)
  }

  return (
    <>
      <div
        className={`video-card relative flex flex-col overflow-hidden rounded-lg border bg-white shadow transition-all dark:bg-ui-surface ${
          checked
            ? 'border-sky-400 ring-2 ring-sky-200 dark:border-sky-400 dark:ring-sky-800'
            : 'border-gray-200 hover:border-gray-300 dark:border-ui-border dark:hover:border-ui-border'
        }`}
      >
        <div className="card-hover-scope video-card-cover group/cover relative aspect-video w-full shrink-0 overflow-hidden bg-gray-200 dark:bg-ui-hover">
          <VideoThumbnail
            src={thumbnailSrc}
            alt={displayName}
            className="h-full w-full object-cover"
            loading="lazy"
          />
          <div
            className="absolute left-2 top-2 z-10 flex items-center gap-1"
            onMouseLeave={() => {
              setFavoriteRatingEditing(false)
              setFavoriteRatingPreview(null)
            }}
            onBlur={(event) => {
              if (event.currentTarget.contains(event.relatedTarget)) return
              setFavoriteRatingEditing(false)
              setFavoriteRatingPreview(null)
            }}
          >
            <Tooltip
              title={
                favoriteRatingError ||
                (favoriteRatingPreview === 0
                  ? zh('清空喜爱度', 'Clear favorite rating')
                  : hasFavoriteRatingTooltipValue
                    ? zh(
                        `喜爱度：${favoriteRatingTooltipValue.toFixed(1)} 分`,
                        `Favorite rating: ${favoriteRatingTooltipValue.toFixed(1)}`
                      )
                    : zh('设置喜爱度评分', 'Set favorite rating'))
              }
              placement="top"
              arrow
            >
              <span
                role="group"
                aria-label={zh('喜爱度评分', 'Favorite rating')}
                className={`flex items-center rounded-full bg-black/70 px-1.5 py-0.5 shadow-lg shadow-black/50 transition-opacity ${
                  favoriteRatingSaving
                    ? 'opacity-60'
                    : favoriteRating > 0
                      ? 'opacity-100'
                      : 'card-hover-focus-visible opacity-0 group-hover/cover:opacity-100'
                }`}
              >
                <span
                  className="flex overflow-hidden transition-[width] duration-150"
                  style={{ width: favoriteRatingWidth * (favoriteRatingIconSize / 21) }}
                >
                  <Rating
                    name={`video-favorite-rating-${video?.id || 'unknown'}`}
                    value={favoriteRating}
                    precision={0.5}
                    size="small"
                    icon={<FavoriteRoundedIcon fontSize="inherit" />}
                    emptyIcon={<FavoriteBorderRoundedIcon fontSize="inherit" />}
                    disabled={favoriteRatingSaving || !video?.id}
                    onChange={handleFavoriteRatingChange}
                    onClick={(event) => event.stopPropagation()}
                    onMouseDown={(event) => event.stopPropagation()}
                    onMouseEnter={() => setFavoriteRatingEditing(true)}
                    onFocus={() => setFavoriteRatingEditing(true)}
                    onChangeActive={(_, value) =>
                      setFavoriteRatingPreview(value >= 0.5 ? value : null)
                    }
                    sx={{
                      flexShrink: 0,
                      color: '#fbbf24',
                      fontSize: favoriteRatingIconSize,
                      '& .MuiRating-iconEmpty': {
                        color: 'rgba(255,255,255,0.85)',
                      },
                    }}
                  />
                </span>
                {favoriteRatingEditing && favoriteRating > 0 ? (
                  <button
                    type="button"
                    className="ml-1 flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-white transition hover:bg-white/20"
                    disabled={favoriteRatingSaving || !video?.id}
                    aria-label={zh('清除喜爱度评分', 'Clear favorite rating')}
                    onMouseEnter={() => setFavoriteRatingPreview(0)}
                    onMouseLeave={() => setFavoriteRatingPreview(null)}
                    onMouseDown={(event) => event.stopPropagation()}
                    onClick={(event) => handleFavoriteRatingChange(event, 0)}
                  >
                    <RemoveCircleOutlineRoundedIcon sx={{ fontSize: 15 }} />
                  </button>
                ) : null}
                {favoriteRating > 0 && !favoriteRatingEditing ? (
                  <span className="ml-1 shrink-0 text-xs font-semibold tabular-nums leading-none text-white">
                    {favoriteRating.toFixed(1)}
                  </span>
                ) : null}
              </span>
            </Tooltip>
            {showSelection ? (
              <Tooltip
                title={checked ? zh('取消选择', 'Deselect') : zh('选择', 'Select')}
                placement="top"
                arrow
              >
                <button
                  type="button"
                  role="checkbox"
                  aria-checked={checked}
                  aria-label={zh(`选择 ${displayName}`, `Select ${displayName}`)}
                  onKeyDown={(event) => {
                    if (event.key === ' ' || event.key === 'Enter') event.stopPropagation()
                  }}
                  onClick={(event) => {
                    event.stopPropagation()
                    onToggle?.()
                  }}
                  className={`card-hover-focus-visible flex h-8 w-8 shrink-0 items-center justify-center rounded-full bg-black/65 text-white shadow-lg shadow-black/40 transition hover:bg-black/80 [@media(hover:none)]:opacity-100 ${
                    checked ? 'text-sky-300 opacity-100' : 'opacity-0 group-hover/cover:opacity-100'
                  }`}
                >
                  {checked ? (
                    <CheckBoxRoundedIcon sx={{ fontSize: 18 }} />
                  ) : (
                    <CheckBoxOutlineBlankRoundedIcon sx={{ fontSize: 18 }} />
                  )}
                </button>
              </Tooltip>
            ) : null}
          </div>
          <button
            type="button"
            className={`card-modal-trigger card-hover-focus-visible absolute right-2 top-2 z-10 flex h-8 w-8 items-center justify-center rounded-full shadow-lg shadow-black/40 transition ${
              favoriteCount > 0
                ? 'bg-amber-400 text-amber-950 hover:bg-amber-300'
                : 'bg-black/65 text-white opacity-0 hover:bg-black/80 group-hover/cover:opacity-100'
            }`}
            title={zh('视频收藏夹', 'Video favorite groups')}
            aria-label={zh('视频收藏夹', 'Video favorite groups')}
            onClick={handleOpenFavorites}
          >
            {favoriteCount > 0 ? (
              <StarRoundedIcon sx={{ fontSize: 18 }} />
            ) : (
              <StarBorderRoundedIcon sx={{ fontSize: 18 }} />
            )}
          </button>
          {javCode ? (
            <div
              className={`absolute right-12 top-2 z-10 max-w-[calc(100%-4.5rem)] truncate rounded bg-black/70 px-2 py-1 text-xs text-white transition-opacity ${
                favoriteRatingEditing ? 'pointer-events-none opacity-0' : 'opacity-100'
              }`}
              title={zh(`已刮削：${javCode}`, `Scraped: ${javCode}`)}
            >
              {zh(`已刮削 ${javCode}`, `Scraped ${javCode}`)}
            </div>
          ) : null}
          <div className="card-hover-focus-visible absolute bottom-2 left-2 z-10 opacity-0 transition-opacity group-hover/cover:opacity-100">
            <button
              type="button"
              onClick={handleOpenScreenshots}
              title={zh('查看截图', 'View screenshots')}
              aria-label={zh('查看截图', 'View screenshots')}
              className={coverButtonClassName}
            >
              <PhotoLibraryOutlinedIcon className="h-4 w-4 text-white" fontSize="inherit" />
            </button>
          </div>
          <div className="card-hover-focus-visible absolute bottom-2 right-2 z-10 flex items-center gap-1 opacity-0 transition-opacity group-hover/cover:opacity-100">
            {showTagEditor ? (
              <Tooltip title={zh('修改标签', 'Edit tags')}>
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation()
                    onOpenTagPicker()
                  }}
                  aria-label={zh('修改标签', 'Edit tags')}
                  className={coverButtonClassName}
                >
                  <LocalOfferOutlinedIcon className="h-4 w-4" fontSize="inherit" />
                </button>
              </Tooltip>
            ) : null}
            {canOpenFile ? (
              <Tooltip title={openFileLabel || zh('用默认程序打开', 'Open with default app')}>
                <button
                  type="button"
                  onClick={handleOpenFile}
                  disabled={!canOpen}
                  aria-label={openFileLabel || zh('打开文件', 'Open file')}
                  className={coverButtonClassName}
                >
                  <PlayArrowIcon className="h-4 w-4" fontSize="inherit" />
                </button>
              </Tooltip>
            ) : null}
            {canRevealFile ? (
              <Tooltip
                title={
                  <span className="whitespace-normal break-all">
                    {zh(`打开所在位置：${displayPath}`, `Reveal in folder: ${displayPath}`)}
                  </span>
                }
              >
                <button
                  type="button"
                  onClick={handleRevealFile}
                  disabled={!canOpen}
                  aria-label={zh('打开所在位置', 'Reveal in folder')}
                  className={coverButtonClassName}
                >
                  <FolderOpenIcon className="h-4 w-4" fontSize="inherit" />
                </button>
              </Tooltip>
            ) : null}
            <Tooltip title={zh('编辑视频', 'Edit video')}>
              <button
                type="button"
                onClick={openEditMenu}
                aria-label={zh('编辑视频', 'Edit video')}
                className={coverButtonClassName}
              >
                <MovieEdit className="h-4 w-4" fontSize="inherit" />
              </button>
            </Tooltip>
            <Tooltip title={zh('刮削设置', 'Scrape settings')}>
              <button
                type="button"
                onClick={handleOpenScrapeSettings}
                aria-label={zh('刮削设置', 'Scrape settings')}
                className={coverButtonClassName}
              >
                <ManageSearchIcon
                  className={`h-4 w-4 ${hasScrapeOverride ? 'text-blue-300' : ''}`}
                  fontSize="inherit"
                />
              </button>
            </Tooltip>
          </div>
          <div className="card-hover-focus-visible pointer-events-none absolute inset-0 flex items-center justify-center bg-black/0 text-white opacity-0 transition-opacity group-hover/cover:opacity-100">
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation()
                e.currentTarget.blur()
                onPlay(video)
              }}
              onPointerUp={(e) => {
                e.currentTarget.blur()
              }}
              className="pointer-events-auto rounded-full bg-black/60 p-2.5 hover:bg-black/80"
              aria-label={zh('播放', 'Play')}
              title={zh('播放', 'Play')}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                viewBox="0 0 24 24"
                fill="currentColor"
                className="h-9 w-9"
              >
                <path d="M8 5v14l11-7z" />
              </svg>
            </button>
          </div>
        </div>

        <div className="p-3">
          <div
            className="line-clamp-2 text-[11px] font-medium leading-snug sm:text-xs"
            title={displayName}
          >
            {displayName}
          </div>
          {videoPath ? (
            <div
              className="mt-1 truncate text-[10px] text-gray-500 dark:text-ui-faint sm:text-[11px]"
              title={relativeDirectoryPath}
            >
              {directoryId ? (
                <a
                  href={directoryHref}
                  onClick={handleDirectoryClick}
                  className="hover:text-ui-primary-hover hover:underline dark:hover:text-blue-300"
                  aria-label={zh(
                    `跳转到目录 ${relativeDirectoryPath}`,
                    `Go to directory ${relativeDirectoryPath}`
                  )}
                >
                  {relativeDirectoryPath}
                </a>
              ) : (
                relativeDirectoryPath
              )}
            </div>
          ) : null}
          <div className="mt-2 flex min-w-0 items-start gap-2">
            <div className="flex min-w-0 flex-1 flex-wrap items-center gap-1">
              <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700 dark:bg-ui-muted dark:text-ui-secondary">
                {durationMinutes
                  ? zh(`${durationMinutes} 分钟`, `${durationMinutes} min`)
                  : zh('时长未知', 'Unknown duration')}
              </span>
              {resolution ? (
                <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700 dark:bg-ui-muted dark:text-ui-secondary">
                  {resolution}
                </span>
              ) : null}
              {sizeText ? (
                <span className="inline-flex h-4 items-center rounded bg-gray-100 px-1 text-[10px] font-medium text-gray-700 dark:bg-ui-muted dark:text-ui-secondary">
                  {sizeText}
                </span>
              ) : null}
            </div>
            <WatchTimeIcons watchedMs={video?.watched_ms} entityType="videos" entityId={video.id} />
          </div>
          {video.tags?.length ? (
            <div className="mt-2 flex flex-wrap items-center gap-1">
              {video.tags.map((t) => (
                <button
                  key={t.id}
                  type="button"
                  className="video-card-tag inline-flex h-4 items-center justify-center rounded px-1 text-[10px] font-semibold"
                  onClick={(e) => {
                    e.stopPropagation()
                    onTagClick?.(t.name)
                  }}
                >
                  {t.name}
                </button>
              ))}
            </div>
          ) : null}
        </div>
        <Popover
          open={Boolean(editAnchorEl)}
          anchorEl={editAnchorEl}
          onClose={closeEditMenu}
          disableScrollLock
          anchorOrigin={{ vertical: 'center', horizontal: 'right' }}
          transformOrigin={{ vertical: 'center', horizontal: 'left' }}
        >
          <div className="flex min-w-[112px] flex-col p-1">
            <button
              type="button"
              onClick={handleRename}
              className="inline-flex items-center gap-2 rounded px-2 py-1.5 text-left text-xs text-gray-700 hover:bg-gray-50 dark:text-ui-secondary dark:hover:bg-ui-hover"
            >
              <DriveFileRenameOutlineIcon className="h-4 w-4" fontSize="inherit" />
              <span>{zh('重命名', 'Rename')}</span>
            </button>
            <button
              type="button"
              onClick={handleDelete}
              className="inline-flex items-center gap-2 rounded px-2 py-1.5 text-left text-xs text-red-600 hover:bg-red-50 dark:text-red-300 dark:hover:bg-red-950"
            >
              <DeleteOutlineIcon className="h-4 w-4" fontSize="inherit" />
              <span>{zh('删除', 'Delete')}</span>
            </button>
          </div>
        </Popover>
      </div>
      <JavFavoriteModal
        open={favoritesOpen}
        idol={{ id: video.id, name: displayName }}
        entityType="video"
        groups={favoriteGroups}
        selectedIds={favoriteSelectedIds}
        loading={favoriteLoading}
        saving={favoriteSaving}
        error={favoriteError}
        onClose={() => setFavoritesOpen(false)}
        onCreateGroup={async (name) => {
          const group = await createJavFavoriteGroup('video', name)
          setFavoriteGroups((current) => [...current, group])
          return group
        }}
        onSave={handleSaveFavorites}
      />
    </>
  )
}
