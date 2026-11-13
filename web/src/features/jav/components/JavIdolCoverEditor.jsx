import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'
import JavActorCoverModal from '@/features/jav/components/JavActorCoverModal'
import { actorGenderOf, ACTOR_GENDER_MALE } from '@/utils/actorTabs'

// Male actors have a dedicated cover editor (local upload / image URL, like the
// JAV edit modal). Female idols keep their work-based cover picker with crop.
export default function JavIdolCoverEditor({ gender, item, ...rest }) {
  const resolved = actorGenderOf(gender, item)
  if (resolved === ACTOR_GENDER_MALE) {
    return <JavActorCoverModal item={item} {...rest} />
  }
  return <JavIdolCoverModal gender={resolved} item={item} {...rest} />
}
