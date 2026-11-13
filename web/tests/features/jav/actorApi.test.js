import assert from 'node:assert/strict'
import test from 'node:test'
import { loadModules } from '../../helpers/modules.js'

// The male actor feature reuses the idol table and handlers but routes through
// /jav/actors so a female endpoint never touches a male row. These tests pin the
// URL routing so a refactor cannot silently send a male request to /jav/idols.
async function loadApi(t) {
  const [
    {
      fetchJavIdols,
      createJavIdol,
      deleteJavIdol,
      mergeJavIdols,
      updateJavIdol,
      fetchJavIdolOptions,
      fetchJavIdolCoverOptions,
      updateJavIdolCover,
      fetchJavIdolPreview,
      resolveJavIdols,
    },
  ] = await loadModules(t, ['features/jav/api.js'])
  return {
    fetchJavIdols,
    createJavIdol,
    deleteJavIdol,
    mergeJavIdols,
    updateJavIdol,
    fetchJavIdolOptions,
    fetchJavIdolCoverOptions,
    updateJavIdolCover,
    fetchJavIdolPreview,
    resolveJavIdols,
  }
}

function mockFetch(t, onRequest) {
  t.mock.method(globalThis, 'fetch', async (url, init) => {
    if (onRequest) onRequest(url, init)
    return Response.json({ items: [], id: 1, ok: true })
  })
}

test('female (default) idol calls use /jav/idols', async (t) => {
  const api = await loadApi(t)
  let url = ''
  mockFetch(t, (u) => {
    url = String(u)
  })
  await api.fetchJavIdols({ gender: 'female' })
  assert.ok(url.includes('/jav/idols'), `expected /jav/idols, got ${url}`)
  await api.fetchJavIdols()
  assert.ok(url.includes('/jav/idols'), `default should hit /jav/idols, got ${url}`)
  await api.createJavIdol('Alice', 'female')
  assert.ok(url.includes('/jav/idols'), `create should hit /jav/idols, got ${url}`)
})

test('male actor calls use /jav/actors', async (t) => {
  const api = await loadApi(t)
  let url = ''
  mockFetch(t, (u) => {
    url = String(u)
  })
  await api.fetchJavIdols({ gender: 'male' })
  assert.ok(url.includes('/jav/actors'), `expected /jav/actors, got ${url}`)
  await api.createJavIdol('Bob', 'male')
  assert.ok(url.includes('/jav/actors'), `create should hit /jav/actors, got ${url}`)
  await api.deleteJavIdol(7, 'male')
  assert.ok(url.includes('/jav/actors/7'), `delete should hit /jav/actors/7, got ${url}`)
  await api.mergeJavIdols({ canonicalId: 1, mergeIds: [2, 3], gender: 'male' })
  assert.ok(url.includes('/jav/actors/merge'), `merge should hit /jav/actors/merge, got ${url}`)
  await api.updateJavIdol(4, { name: 'Bob' }, 'male')
  assert.ok(url.includes('/jav/actors/4'), `update should hit /jav/actors/4, got ${url}`)
  await api.fetchJavIdolOptions({ gender: 'male' })
  assert.ok(
    url.includes('/jav/actors/options'),
    `options should hit /jav/actors/options, got ${url}`
  )
  await api.fetchJavIdolCoverOptions(5, 'male')
  assert.ok(
    url.includes('/jav/actors/5/cover-options'),
    `cover-options should hit /jav/actors/5/cover-options, got ${url}`
  )
  await api.updateJavIdolCover(5, { javId: 9 }, 'male')
  assert.ok(url.includes('/jav/actors/5/cover'), `cover should hit /jav/actors/5/cover, got ${url}`)
  await api.fetchJavIdolPreview(6, 'male')
  assert.ok(url.includes('/jav/actors/6'), `preview should hit /jav/actors/6, got ${url}`)
})

test('resolve keeps male and female requests on separate in-flight keys', async (t) => {
  const api = await loadApi(t)
  const seen = []
  mockFetch(t, (u) => {
    seen.push(String(u))
  })
  await Promise.all([
    api.resolveJavIdols([10, 11], 'male'),
    api.resolveJavIdols([10, 11], 'female'),
  ])
  assert.equal(seen.length, 2, `expected two resolve calls, got ${seen.length}`)
  assert.ok(
    seen.some((u) => u.includes('/jav/actors/resolve')),
    'missing male resolve'
  )
  assert.ok(
    seen.some((u) => u.includes('/jav/idols/resolve')),
    'missing female resolve'
  )
})

test('male and female idol list params both reach the correct endpoint', async (t) => {
  const api = await loadApi(t)
  const urls = []
  mockFetch(t, (u) => {
    urls.push(String(u))
  })
  await api.fetchJavIdols({ search: 'x', gender: 'male' })
  await api.fetchJavIdols({ search: 'x', gender: 'female' })
  assert.ok(urls[0].startsWith('/jav/actors?'), `male list url: ${urls[0]}`)
  assert.ok(urls[1].startsWith('/jav/idols?'), `female list url: ${urls[1]}`)
})
