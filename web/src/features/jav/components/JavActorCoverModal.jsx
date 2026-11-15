import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'

// Male actors use the exact same cover editor as female idols: a work cover
// list, a large preview with a draggable crop selection, a start slider and
// reset/save buttons. The only extra is the upload entry, enabled here.
export default function JavActorCoverModal(props) {
  return <JavIdolCoverModal allowUpload {...props} />
}
