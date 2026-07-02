import { api } from '@/lib/api'
import type { VideoChannel, VideoChannelFormValues } from './types'

export async function getVideoChannels(): Promise<VideoChannel[]> {
  const res = await api.get<{ code: number; data: VideoChannel[] }>(
    '/api/video-generation/channels'
  )
  return res.data.data ?? []
}

export async function createVideoChannel(
  data: VideoChannelFormValues
): Promise<VideoChannel> {
  const res = await api.post<{ code: number; data: VideoChannel }>(
    '/api/video-generation/channels',
    data
  )
  return res.data.data
}

export async function updateVideoChannel(
  id: number,
  data: VideoChannelFormValues
): Promise<VideoChannel> {
  const res = await api.put<{ code: number; data: VideoChannel }>(
    `/api/video-generation/channels/${id}`,
    data
  )
  return res.data.data
}

export async function deleteVideoChannel(id: number): Promise<void> {
  await api.delete(`/api/video-generation/channels/${id}`)
}

export async function updateVideoChannelStatus(
  id: number,
  enabled: number
): Promise<void> {
  await api.put(`/api/video-generation/channels/${id}/status`, { enabled })
}

export const videoChannelsQueryKeys = {
  all: ['video-channels'] as const,
  list: () => [...videoChannelsQueryKeys.all, 'list'] as const,
}
