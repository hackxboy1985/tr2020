/* eslint-disable react-refresh/only-export-components */
import React, { createContext, useContext, useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { videoChannelsQueryKeys } from '../api'
import type { VideoChannel } from '../types'

type DialogType = 'create' | 'update' | 'delete' | null

type VideoChannelsContextType = {
  open: DialogType
  setOpen: (open: DialogType) => void
  currentRow: VideoChannel | null
  setCurrentRow: (row: VideoChannel | null) => void
  refresh: () => void
}

const VideoChannelsContext = createContext<VideoChannelsContextType | undefined>(
  undefined
)

export function VideoChannelsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useState<DialogType>(null)
  const [currentRow, setCurrentRow] = useState<VideoChannel | null>(null)
  const queryClient = useQueryClient()

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: videoChannelsQueryKeys.all })
  }

  return (
    <VideoChannelsContext.Provider
      value={{ open, setOpen, currentRow, setCurrentRow, refresh }}
    >
      {children}
    </VideoChannelsContext.Provider>
  )
}

export function useVideoChannels() {
  const ctx = useContext(VideoChannelsContext)
  if (!ctx)
    throw new Error('useVideoChannels must be used within VideoChannelsProvider')
  return ctx
}
