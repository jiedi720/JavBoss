import JavIdolCoverModal from '@/features/jav/components/JavIdolCoverModal'

// Male actors keep the exact same editor UI and interactions as female idols
// (preview, draggable crop box, start slider, reset/save), plus an upload entry.
// A work cover never contains the male actor, so the work-cover list is hidden
// and the cover comes from the uploaded image / current cover only.
export default function JavActorCoverModal(props) {
  return <JavIdolCoverModal {...props} allowUpload showWorkCovers={false} />
}
