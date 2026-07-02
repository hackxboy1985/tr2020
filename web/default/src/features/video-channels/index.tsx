import { useTranslation } from 'react-i18next'
import { SectionPageLayout } from '@/components/layout'
import { VideoChannelsProvider } from './components/video-channels-provider'
import { VideoChannelsTable } from './components/video-channels-table'
import { VideoChannelsDialogs } from './components/video-channels-dialogs'
import { VideoChannelsPrimaryButtons } from './components/video-channels-primary-buttons'

export function VideoChannels() {
  const { t } = useTranslation()
  return (
    <VideoChannelsProvider>
      <SectionPageLayout>
        <SectionPageLayout.Title>
          {t('Video Generation Channels')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <VideoChannelsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <VideoChannelsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <VideoChannelsDialogs />
    </VideoChannelsProvider>
  )
}
