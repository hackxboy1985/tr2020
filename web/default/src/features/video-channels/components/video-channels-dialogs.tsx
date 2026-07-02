import { useVideoChannels } from './video-channels-provider'
import { VideoChannelMutateDrawer } from './video-channel-mutate-drawer'

export function VideoChannelsDialogs() {
  const { open, setOpen, currentRow } = useVideoChannels()
  return (
    <VideoChannelMutateDrawer
      open={open === 'create' || open === 'update'}
      onOpenChange={(v) => !v && setOpen(null)}
      currentRow={open === 'update' ? currentRow : null}
    />
  )
}
