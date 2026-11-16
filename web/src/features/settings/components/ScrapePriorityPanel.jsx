import { useEffect, useState } from 'react'
import { fetchConfig, updateConfig } from '@/features/settings/api'
import { zh } from '@/utils/i18n'
import { getErrorMessage } from '@/utils/errors'

const CATEGORIES = [
  { key: 'censored', label: ['有码', 'Censored'] },
  { key: 'uncensored', label: ['无码', 'Uncensored'] },
  { key: 'amateur', label: ['素人', 'Amateur'] },
  { key: 'fc2', label: ['FC2', 'FC2'] },
]

const SOURCE_NAMES = {
  dmm: 'DMM',
  mgstage: 'Mgstage',
  javlibrary: 'JavLibrary',
  avbase: 'Avbase',
  hbox_jp: 'Hbox JP',
  javdb: 'JavDB',
  javbus: 'JavBus',
  jav321: 'Jav321',
  avmoo: 'Avmoo',
  mmtv: 'MMTV',
  airav_io: 'AirAV',
  freejavbt: 'FreeJavBT',
  miss_av: 'MissAV',
  carib: 'Carib',
  avsox: 'Avsox',
  fc2: 'FC2',
  fc2_hub: 'FC2 Hub',
}

const LIVE_SOURCES = new Set(['javdb', 'javbus', 'avmoo', 'avsox'])
const configKey = (category) => `jav_scrape_priority_${category}`

export default function ScrapePriorityPanel() {
  const [lists, setLists] = useState(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    let cancelled = false
    fetchConfig()
      .then((cfg) => {
        if (cancelled) return
        setLists(
          Object.fromEntries(
            CATEGORIES.map((c) => [
              c.key,
              String(cfg[configKey(c.key)] || '')
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean),
            ])
          )
        )
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
  }, [])

  const move = (category, index, delta) => {
    setLists((prev) => {
      const list = [...(prev[category] || [])]
      const next = index + delta
      if (next < 0 || next >= list.length) return prev
      ;[list[index], list[next]] = [list[next], list[index]]
      return { ...prev, [category]: list }
    })
    setSaved(false)
  }

  const save = async () => {
    if (!lists || saving) return
    setSaving(true)
    setError('')
    setSaved(false)
    try {
      const payload = Object.fromEntries(
        CATEGORIES.map((c) => [configKey(c.key), lists[c.key].join(',')])
      )
      await updateConfig(payload)
      setSaved(true)
    } catch (err) {
      setError(getErrorMessage(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section className="rounded-2xl border border-zinc-200 bg-white p-5 shadow-sm dark:border-ui-border dark:bg-ui-surface">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h4 className="text-[13px] font-semibold text-zinc-800 dark:text-ui-text">
            {zh('刮削源优先级', 'Scrape Source Priority')}
          </h4>
          <p className="mt-1 text-xs leading-5 text-zinc-500 dark:text-ui-faint">
            {zh(
              '按番号类型从高到低依次尝试；标“已接入”的源会真正联网查询，其余为预留。',
              'Sources are tried top to bottom per code type; only connected sources query the network.'
            )}
          </p>
        </div>
        <button
          type="button"
          onClick={save}
          disabled={loading || saving || !lists}
          className="rounded-lg bg-ui-primary px-3 py-1.5 text-xs font-medium text-white transition hover:bg-ui-primary-hover disabled:opacity-60"
        >
          {saving ? zh('保存中…', 'Saving...') : zh('保存顺序', 'Save order')}
        </button>
      </div>

      {loading && (
        <p className="mt-4 text-sm text-zinc-500 dark:text-ui-faint">
          {zh('加载中…', 'Loading...')}
        </p>
      )}
      {error && <p className="mt-3 text-sm text-red-600 dark:text-red-400">{error}</p>}
      {saved && <p className="mt-3 text-sm text-emerald-600">{zh('已保存', 'Saved')}</p>}

      {lists && (
        <div className="mt-4 grid gap-4 md:grid-cols-2">
          {CATEGORIES.map((c) => (
            <div
              key={c.key}
              className="rounded-xl border border-zinc-200 p-3 dark:border-ui-border"
            >
              <div className="mb-2 text-xs font-semibold text-zinc-700 dark:text-ui-secondary">
                {zh(c.label[0], c.label[1])}
              </div>
              <div className="space-y-1">
                {(lists[c.key] || []).map((source, index) => {
                  const live = LIVE_SOURCES.has(source)
                  const total = (lists[c.key] || []).length
                  return (
                    <div
                      key={source}
                      className="flex items-center gap-2 rounded-lg bg-zinc-50 px-2 py-1 dark:bg-ui-surface/50"
                    >
                      <span className="w-4 text-right text-[11px] text-zinc-400">{index + 1}</span>
                      <span className="flex-1 text-xs text-zinc-700 dark:text-ui-text">
                        {SOURCE_NAMES[source] || source}
                      </span>
                      <span
                        className={`text-[10px] ${live ? 'text-emerald-600' : 'text-zinc-400'}`}
                      >
                        {live ? zh('已接入', 'Active') : zh('预留', 'Planned')}
                      </span>
                      <button
                        type="button"
                        onClick={() => move(c.key, index, -1)}
                        disabled={index === 0}
                        className="rounded px-1.5 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:text-ui-faint dark:hover:bg-ui-border"
                        aria-label="move up"
                      >
                        ↑
                      </button>
                      <button
                        type="button"
                        onClick={() => move(c.key, index, 1)}
                        disabled={index === total - 1}
                        className="rounded px-1.5 text-xs text-zinc-500 hover:bg-zinc-200 disabled:opacity-30 dark:text-ui-faint dark:hover:bg-ui-border"
                        aria-label="move down"
                      >
                        ↓
                      </button>
                    </div>
                  )
                })}
              </div>
            </div>
          ))}
        </div>
      )}
    </section>
  )
}
