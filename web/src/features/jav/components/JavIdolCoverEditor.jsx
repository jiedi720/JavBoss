import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'
import JavActorCoverModal from '@/features/jav/components/JavActorCoverModal'
import { actorGenderOf, ACTOR_GENDER_MALE } from '@/utils/actorTabs'

// Male actors use the same work-based cover picker and crop UI as female idols;
// their editor only adds an upload entry on top of it.
export default function JavIdolCoverEditor({ gender, item, ...rest }) {
  const resolved = actorGenderOf(gender, item)
  if (resolved === ACTOR_GENDER_MALE) {
    return <JavActorCoverModal gender={resolved} item={item} {...rest} />
  }
  return <JavIdolCoverModal gender={resolved} item={item} {...rest} />
}
