import { useCallback, useEffect, useMemo, useState } from 'react'
import CloseOutlinedIcon from '@mui/icons-material/CloseOutlined'
import OpenInNewRoundedIcon from '@mui/icons-material/OpenInNewRounded'
import AppModal from '@/shared/ui/AppModal'
import {
  fetchJavActressProfile,
  fillJavActressProfile,
  lookupJavActresses,
} from '@/features/jav/api'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

// Look-up de-duplication mirrors the userscript: answering "no" to a name
// mismatch silences that work code forever, answering "yes" silences the
// code + current actress combination for a week.
const MISMATCH_DONE_KEY = 'javboss_code_mismatch_done'
const MISMATCH_NEVER_KEY = 'javboss_code_mismatch_never'
const MISMATCH_TTL = 7 * 24 * 60 * 60 * 1000

const CUP_LETTERS = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'

const PROFILE_FIELDS = [
  ['japanese_name', '日文名', 'Japanese name'],
  ['roman_name', '罗马名', 'Roman name'],
  ['chinese_name', '中文名', 'Chinese name'],
  ['height_cm', '身高', 'Height'],
  ['birth_date', '生日', 'Birth date'],
  ['bust', '胸围', 'Bust'],
  ['waist', '腰围', 'Waist'],
  ['hips', '臀围', 'Hips'],
  ['cup', '罩杯', 'Cup'],
  ['aliases', '别名', 'Aliases'],
]

const FILL_LABELS = {
  japanese_name: ['日文名', 'Japanese name'],
  roman_name: ['罗马名', 'Roman name'],
  chinese_name: ['中文名', 'Chinese name'],
  height_cm: ['身高', 'Height'],
  birth_date: ['生日', 'Birth date'],
  bust: ['胸围', 'Bust'],
  waist: ['腰围', 'Waist'],
  hips: ['臀围', 'Hips'],
  cup: ['罩杯', 'Cup'],
}

function loadJson(key) {
  try {
    return JSON.parse(localStorage.getItem(key) || '{}')
  } catch {
    return {}
  }
}

function saveJson(key, value) {
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    // Storage can be unavailable in private mode; the session still works.
  }
}

function compactName(value) {
  return String(value || '')
    .replace(/\s+/g, '')
    .toLowerCase()
}

function namesMatch(a, b) {
  const left = compactName(a)
  const right = compactName(b)
  if (!left || !right) return false
  return left === right || left.includes(right) || right.includes(left)
}

function neverKey(code) {
  return `NEVER|${String(code || '')
    .toUpperCase()
    .trim()}`
}

function onceKey(code, names) {
  return `ONCE|${String(code || '')
    .toUpperCase()
    .trim()}|${(names || []).join('/')}`
}

// Same DMM rule as the external link: the cid is the lower-case code without
// separators (HMN-929 -> hmn929).
function dmmURL(code) {
  const raw = String(code || '').trim()
  if (!raw) return ''
  if (/^[A-Z0-9]+-?\d+$/i.test(raw)) {
    return `https://www.dmm.co.jp/mono/dvd/-/detail/=/cid=${raw.toLowerCase().replace(/[^a-z0-9]/g, '')}/`
  }
  return `https://www.dmm.co.jp/search/=/searchstr=${encodeURIComponent(raw)}/`
}

function minnanoURL(name) {
  return `https://www.minnano-av.com/search_result.php?search_scope=actress&search_word=${encodeURIComponent(
    name || ''
  )}&search=+Go+`
}

function formatCup(value) {
  const n = Number(value)
  if (!Number.isFinite(n) || n <= 0) return ''
  return `${CUP_LETTERS[n - 1] || n}`
}

function profileValue(profile, key) {
  if (!profile) return ''
  if (key === 'aliases') {
    const list = Array.isArray(profile.aliases) ? profile.aliases : []
    return list.join('、')
  }
  if (key === 'cup') return formatCup(profile.cup)
  return String(profile[key] ?? '')
}

// Code-driven actress lookup. It replaces the browser userscript feature with a
// native flow: list the actresses credited on the work, preview a scraped
// profile, then write it onto the idol (creating and linking it when needed).
export default function ActressLookupModal({
  open,
  code,
  javId,
  currentIdolNames = [],
  onClose,
  onFilled,
}) {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [actresses, setActresses] = useState([])
  const [source, setSource] = useState('')
  const [preview, setPreview] = useState(null)
  const [previewLoading, setPreviewLoading] = useState('')
  const [overwrite, setOverwrite] = useState(false)
  const [saving, setSaving] = useState(false)
  const [result, setResult] = useState(null)
  const [mismatchAck, setMismatchAck] = useState('')
  const [decisionVersion, setDecisionVersion] = useState(0)

  const normalizedCode = String(code || '')
    .trim()
    .toUpperCase()
  const femaleNames = useMemo(
    () => (currentIdolNames || []).map((name) => String(name || '').trim()).filter(Boolean),
    [currentIdolNames]
  )

  const load = useCallback(async () => {
    if (!normalizedCode) return
    setLoading(true)
    setError('')
    setActresses([])
    setPreview(null)
    setResult(null)
    setMismatchAck('')
    try {
      const data = await lookupJavActresses(normalizedCode)
      setActresses(Array.isArray(data?.actresses) ? data.actresses : [])
      setSource(String(data?.source || 'avwiki'))
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setLoading(false)
    }
  }, [normalizedCode])

  useEffect(() => {
    if (!open) return
    setOverwrite(false)
    setSaving(false)
    setPreviewLoading('')
    void load()
  }, [open, load])

  const mismatch = useMemo(() => {
    if (!femaleNames.length || !actresses.length) return false
    return !actresses.some((item) => femaleNames.some((name) => namesMatch(item.name, name)))
  }, [actresses, femaleNames])

  // "No" silences this code forever; "yes" silences the code + current actress
  // combination for a week. Both are persisted, so a reopened modal keeps them.
  const mutedByStorage = useMemo(() => {
    // decisionVersion only forces a re-read after the user answers.
    void decisionVersion
    if (!normalizedCode) return false
    return Boolean(loadJson(MISMATCH_NEVER_KEY)[neverKey(normalizedCode)])
  }, [normalizedCode, decisionVersion])

  const acceptedByStorage = useMemo(() => {
    void decisionVersion
    if (!normalizedCode) return false
    const item = loadJson(MISMATCH_DONE_KEY)[onceKey(normalizedCode, femaleNames)]
    return Boolean(item && Date.now() - Number(item.ts || 0) < MISMATCH_TTL)
  }, [normalizedCode, femaleNames, decisionVersion])

  const muted = mutedByStorage || mismatchAck === 'no'
  const accepted = acceptedByStorage || mismatchAck === 'yes'
  const needsMismatchAnswer = mismatch && !muted && !accepted
  const fillBlocked = mismatch && (muted || needsMismatchAnswer)

  const answerMismatch = (accepted) => {
    setMismatchAck(accepted ? 'yes' : 'no')
    if (accepted) {
      const key = onceKey(normalizedCode, femaleNames)
      const map = loadJson(MISMATCH_DONE_KEY)
      map[key] = { ts: Date.now() }
      saveJson(MISMATCH_DONE_KEY, map)
    } else {
      const key = neverKey(normalizedCode)
      const map = loadJson(MISMATCH_NEVER_KEY)
      map[key] = { ts: Date.now() }
      saveJson(MISMATCH_NEVER_KEY, map)
      setPreview(null)
    }
    setDecisionVersion((current) => current + 1)
  }

  const handlePreview = async (actor) => {
    if (!actor || fillBlocked) return
    setPreviewLoading(actor.name)
    setError('')
    setResult(null)
    try {
      const data = await fetchJavActressProfile(actor.name)
      setPreview({
        name: String(data?.name || actor.name),
        source: String(data?.source || ''),
        profile: data?.profile || {},
        profileUrl: String(actor.profile_url || data?.profile?.profile_url || ''),
      })
      setOverwrite(false)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setPreviewLoading('')
    }
  }

  const handleFill = async () => {
    if (!preview || saving || fillBlocked) return
    setSaving(true)
    setError('')
    try {
      const data = await fillJavActressProfile({
        javId: Number(javId) || 0,
        name: preview.name,
        overwrite,
        profile: preview.profile,
      })
      const fields = Array.isArray(data?.filled) ? data.filled : []
      setResult({
        name: String(data?.idol?.name || preview.name),
        idolId: Number(data?.idol?.id) || 0,
        fields,
      })
      if (data?.jav) onFilled?.(data.jav)
      setPreview(null)
      setOverwrite(false)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  if (!open) return null

  return (
    <AppModal
      ariaLabel={zh('番号查女优', 'Look up actresses by code')}
      className="px-4 py-6"
      closeDisabled={saving}
      contentClassName="flex max-h-[90vh] w-full max-w-2xl flex-col overflow-hidden rounded-lg bg-white shadow-2xl dark:bg-ui-surface"
      onClose={onClose}
      zIndex={1700}
    >
      <div className="flex items-start justify-between gap-3 border-b border-slate-200 px-4 py-3 dark:border-ui-border">
        <div className="min-w-0">
          <div className="truncate text-base font-semibold text-slate-950 dark:text-ui-text">
            {zh('番号查女优', 'Look up actresses by code')}
          </div>
          <div className="truncate text-xs text-slate-500 dark:text-ui-faint">
            {normalizedCode
              ? zh(`番号 ${normalizedCode}`, `Code ${normalizedCode}`)
              : zh('未识别番号', 'No code')}
            {source ? zh(` · 来源 ${source}`, ` · source ${source}`) : ''}
          </div>
        </div>
        <button
          type="button"
          className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-slate-500 transition hover:bg-slate-100 dark:text-ui-faint dark:hover:bg-ui-hover"
          onClick={onClose}
          aria-label={zh('关闭', 'Close')}
        >
          <CloseOutlinedIcon sx={{ fontSize: 18 }} />
        </button>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto px-4 py-3">
        {error ? (
          <div className="mb-3 rounded border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-700 dark:border-red-900 dark:bg-red-950 dark:text-red-300">
            {error}
          </div>
        ) : null}

        {loading ? (
          <div className="py-10 text-center text-sm text-slate-500 dark:text-ui-faint">
            {zh('正在查询 AV Wiki…', 'Looking up AV Wiki…')}
          </div>
        ) : null}

        {!loading && mismatch ? (
          <div className="mb-3 rounded border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-800 dark:border-amber-900 dark:bg-amber-950 dark:text-amber-300">
            <div className="font-medium">{zh('女优名不一致', 'Actor name mismatch')}</div>
            <div className="mt-1 leading-5">
              {zh('当前：', 'Current: ')}
              <b>{femaleNames.join(' / ') || zh('无', 'none')}</b>
              <br />
              {zh('识别：', 'Found: ')}
              <b>{actresses.map((item) => item.name).join(' / ')}</b>
            </div>
            {muted ? (
              <div className="mt-1 text-[11px] opacity-80">
                {zh(
                  '已记录：该番号以后不再提示，仅显示查询结果。',
                  'Recorded: this code will not prompt again; results are shown only.'
                )}
              </div>
            ) : accepted ? (
              <div className="mt-1 text-[11px] opacity-80">
                {zh('已确认，可继续回填。', 'Confirmed, filling is allowed.')}
              </div>
            ) : (
              <div className="mt-2 flex flex-wrap gap-2">
                <button
                  type="button"
                  className="rounded border border-amber-400 bg-amber-500 px-2 py-1 text-[11px] font-medium text-white transition hover:bg-amber-600"
                  onClick={() => answerMismatch(true)}
                >
                  {zh('是，继续回填', 'Yes, fill anyway')}
                </button>
                <button
                  type="button"
                  className="rounded border border-amber-300 bg-white px-2 py-1 text-[11px] font-medium text-amber-800 transition hover:bg-amber-100 dark:border-amber-800 dark:bg-transparent dark:text-amber-300"
                  onClick={() => answerMismatch(false)}
                >
                  {zh(
                    '否，仅显示结果（该番号不再提示）',
                    'No, show results only (never ask again)'
                  )}
                </button>
              </div>
            )}
          </div>
        ) : null}

        {!loading && !error && !actresses.length ? (
          <div className="py-10 text-center text-sm text-slate-500 dark:text-ui-faint">
            {zh('没有查到该番号的女优信息', 'No actress was found for this code')}
            <div className="mt-3">
              <button
                type="button"
                className="rounded border border-slate-300 px-3 py-1 text-xs text-slate-600 transition hover:bg-slate-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
                onClick={() => void load()}
              >
                {zh('重试', 'Retry')}
              </button>
            </div>
          </div>
        ) : null}

        {actresses.length ? (
          <ul className="flex flex-col gap-2">
            {actresses.map((actor) => {
              const matched = femaleNames.some((name) => namesMatch(actor.name, name))
              return (
                <li
                  key={`${actor.name}-${actor.profile_url || ''}`}
                  className="rounded border border-slate-200 px-3 py-2 dark:border-ui-border"
                >
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="text-sm font-semibold text-slate-900 dark:text-ui-text">
                      {actor.name}
                    </span>
                    {matched ? (
                      <span className="rounded-full bg-purple-100 px-2 py-0.5 text-[10px] font-medium text-purple-700 dark:bg-purple-950 dark:text-purple-300">
                        {zh('已在作品中', 'Already linked')}
                      </span>
                    ) : null}
                    <span className="flex-1" />
                    {actor.profile_url ? (
                      <a
                        href={actor.profile_url}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-800 dark:text-ui-faint dark:hover:text-ui-text"
                      >
                        {zh('AV Wiki', 'AV Wiki')}
                        <OpenInNewRoundedIcon sx={{ fontSize: 12 }} />
                      </a>
                    ) : null}
                    <a
                      href={minnanoURL(actor.name)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-800 dark:text-ui-faint dark:hover:text-ui-text"
                    >
                      {zh('minnano', 'minnano')}
                      <OpenInNewRoundedIcon sx={{ fontSize: 12 }} />
                    </a>
                    <a
                      href={dmmURL(normalizedCode)}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-1 text-[11px] text-slate-500 hover:text-slate-800 dark:text-ui-faint dark:hover:text-ui-text"
                    >
                      {zh('DMM', 'DMM')}
                      <OpenInNewRoundedIcon sx={{ fontSize: 12 }} />
                    </a>
                    <button
                      type="button"
                      disabled={fillBlocked || previewLoading === actor.name}
                      className="rounded border border-rose-300 bg-rose-50 px-2 py-1 text-[11px] font-medium text-rose-700 transition hover:bg-rose-100 disabled:cursor-not-allowed disabled:opacity-50 dark:border-rose-900 dark:bg-rose-950 dark:text-rose-300"
                      onClick={() => void handlePreview(actor)}
                    >
                      {previewLoading === actor.name
                        ? zh('查询中…', 'Loading…')
                        : zh('查看资料', 'Preview profile')}
                    </button>
                  </div>
                </li>
              )
            })}
          </ul>
        ) : null}

        {preview ? (
          <div className="mt-3 rounded border border-slate-200 dark:border-ui-border">
            <div className="flex flex-wrap items-center gap-2 border-b border-slate-200 px-3 py-2 dark:border-ui-border">
              <span className="text-sm font-semibold text-slate-900 dark:text-ui-text">
                {preview.profile?.japanese_name || preview.name}
              </span>
              {preview.source ? (
                <span className="rounded-full bg-slate-100 px-2 py-0.5 text-[10px] text-slate-600 dark:bg-ui-hover dark:text-ui-secondary">
                  {preview.source}
                </span>
              ) : null}
            </div>
            <dl className="grid grid-cols-2 gap-x-4 gap-y-1.5 px-3 py-2 text-xs sm:grid-cols-3">
              {PROFILE_FIELDS.map(([key, zhLabel, enLabel]) => {
                const value = profileValue(preview.profile, key)
                if (!value) return null
                return (
                  <div key={key} className="flex min-w-0 gap-1">
                    <dt className="shrink-0 text-slate-500 dark:text-ui-faint">
                      {zh(zhLabel, enLabel)}
                    </dt>
                    <dd className="min-w-0 break-words text-slate-800 dark:text-ui-text">
                      {value}
                    </dd>
                  </div>
                )
              })}
            </dl>
            <div className="flex flex-wrap items-center gap-3 border-t border-slate-200 px-3 py-2 dark:border-ui-border">
              <label className="flex items-center gap-1.5 text-[11px] text-slate-600 dark:text-ui-secondary">
                <input
                  type="checkbox"
                  checked={overwrite}
                  onChange={(event) => setOverwrite(event.target.checked)}
                  disabled={saving}
                />
                {zh('覆盖已有字段', 'Overwrite existing fields')}
              </label>
              <span className="flex-1" />
              <button
                type="button"
                className="rounded border border-slate-300 px-2 py-1 text-[11px] text-slate-600 transition hover:bg-slate-50 dark:border-ui-border dark:text-ui-secondary dark:hover:bg-ui-hover"
                onClick={() => setPreview(null)}
                disabled={saving}
              >
                {zh('取消', 'Cancel')}
              </button>
              <button
                type="button"
                className="rounded border border-rose-400 bg-rose-500 px-2.5 py-1 text-[11px] font-medium text-white transition hover:bg-rose-600 disabled:cursor-not-allowed disabled:opacity-60"
                onClick={() => void handleFill()}
                disabled={saving || fillBlocked}
              >
                {saving ? zh('写入中…', 'Saving…') : zh('写入女优资料', 'Save to actress')}
              </button>
            </div>
          </div>
        ) : null}

        {result ? (
          <div className="mt-3 rounded border border-emerald-200 bg-emerald-50 px-3 py-2 text-xs text-emerald-800 dark:border-emerald-900 dark:bg-emerald-950 dark:text-emerald-300">
            <div className="font-medium">
              {zh(`已写入「${result.name}」`, `Saved to "${result.name}"`)}
            </div>
            <div className="mt-1 leading-5">
              {result.fields.length
                ? zh(
                    `更新字段：${result.fields
                      .map((key) => (FILL_LABELS[key] ? FILL_LABELS[key][0] : key))
                      .join('、')}`,
                    `Updated: ${result.fields.join(', ')}`
                  )
                : zh(
                    '所有字段都已一致，本次没有改动。可勾选「覆盖已有字段」后重试。',
                    'Every field already matched; nothing changed. Tick "Overwrite existing fields" to force it.'
                  )}
            </div>
          </div>
        ) : null}
      </div>
    </AppModal>
  )
}
