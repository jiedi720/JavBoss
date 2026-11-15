import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'
import JavActorCoverModal from '@/features/jav/components/JavActorCoverModal'
import { actorGenderOf, ACTOR_GENDER_MALE } from '@/utils/actorTabs'

// Male actors reuse the female cover editor with an upload entry, but without
// the crop-selection UI (their uploaded image is shown whole).
export default function JavIdolCoverEditor({ gender, item, ...rest }) {
  const resolved = actorGenderOf(gender, item)
  if (resolved === ACTOR_GENDER_MALE) {
    return <JavActorCoverModal gender={resolved} item={item} {...rest} />
  }
  return <JavIdolCoverModal gender={resolved} item={item} {...rest} />
}
