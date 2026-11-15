import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'

// Male actors share the female idol cover editor (work list + upload entry),
// but show the whole uploaded image instead of cropping a slice out of a wide
// cover, so the crop-selection UI is turned off for them.
export default function JavActorCoverModal(props) {
  return <JavIdolCoverModal {...props} allowUpload cropEnabled={false} />
}
