// Remembers which actor-list tab the user was browsing before they jumped into a
// filtered JAV list, plus the filters that list was showing. Clearing that actor
// filter should take them back to the list exactly as they left it, instead of
// stranding them on the JAV list with all the conditions wiped.
let origin = null

export function rememberIdolJumpOrigin(tab, filters = null) {
  const value = typeof tab === 'string' ? tab.trim() : ''
  if (!value) {
    origin = null
    return
  }
  origin = {
    tab: value,
    // Snapshot only plain data so the remembered state stays independent from
    // later store updates.
    filters: filters && typeof filters === 'object' ? { ...filters } : null,
  }
}

// Returns the remembered { tab, filters } and forgets it, so the restore
// happens only once. Returns null when nothing was remembered.
export function takeIdolJumpOrigin() {
  const value = origin
  origin = null
  return value
}

export function clearIdolJumpOrigin() {
  origin = null
}
