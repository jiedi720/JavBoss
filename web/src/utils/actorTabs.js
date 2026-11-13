// The female and male actor pages share one list resource, one set of filters,
// pagination and scroll state. Only the backend gender (and a few labels) differ.
// Keeping the two tab ids separate lets the URL express which page is open while
// all existing "idol tab" checks can be widened with isActorListTab().
export const IDOL_TAB = 'idol'
export const ACTOR_TAB = 'actor'
export const ACTOR_GENDER_FEMALE = 'female'
export const ACTOR_GENDER_MALE = 'male'

export function isMaleActor(gender) {
  return gender === ACTOR_GENDER_MALE
}

export function isActorListTab(tab) {
  return tab === IDOL_TAB || tab === ACTOR_TAB
}

export function actorGenderForTab(tab) {
  return tab === ACTOR_TAB ? ACTOR_GENDER_MALE : ACTOR_GENDER_FEMALE
}

// Modals are opened both from the actor list (explicit gender) and from a JAV
// detail page (only the item is known). Keep one rule for both entry points.
export function actorGenderOf(gender, item) {
  if (gender === ACTOR_GENDER_MALE || gender === ACTOR_GENDER_FEMALE) return gender
  return isMaleActor(item?.gender) ? ACTOR_GENDER_MALE : ACTOR_GENDER_FEMALE
}

// Shared copy source so the male page reads like the female page instead of
// hard-coding "女优"/"idol" in dozens of places.
export function actorTerms(gender) {
  const male = isMaleActor(gender)
  return {
    male,
    zhNoun: male ? '男优' : '女优',
    enSingular: male ? 'actor' : 'idol',
    enSingularCap: male ? 'Actor' : 'Idol',
    enPlural: male ? 'actors' : 'idols',
    enPluralCap: male ? 'Actors' : 'Idols',
  }
}
