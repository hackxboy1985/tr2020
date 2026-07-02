import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { useVideoChannels } from './video-channels-provider'

export function VideoChannelsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen } = useVideoChannels()
  return (
    <Button onClick={() => setOpen('create')} size='sm'>
      <Plus className='mr-2 h-4 w-4' />
      {t('Add Channel')}
    </Button>
  )
}
