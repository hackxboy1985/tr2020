/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Copy, Check } from 'lucide-react'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { ScrollArea } from '@/components/ui/scroll-area'
import { formatTimestampToDate } from '@/lib/format'
import type { UsageLog } from '../../data/schema'

interface LogPromptDialogProps {
  log: UsageLog
  open: boolean
  onOpenChange: (open: boolean) => void
}

interface PromptData {
  id: number
  log_id: number
  prompt_text: string
  request_body?: string
  response_body?: string
  created_at: number
}

export function LogPromptDialog({
  log,
  open,
  onOpenChange,
}: LogPromptDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const [loading, setLoading] = useState(false)
  const [promptData, setPromptData] = useState<PromptData | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (open && log?.id) {
      // If prompt_text is already in the log, use it directly
      if (log.prompt_text) {
        setPromptData({
          id: 0,
          log_id: log.id,
          prompt_text: log.prompt_text,
          request_body: log.request_body,
          response_body: log.response_body,
          created_at: log.created_at,
        })
        setLoading(false)
      } else {
        // Otherwise fetch it separately
        setLoading(true)
        setError(null)
        fetch(`/api/log/prompt/${log.id}`)
          .then((res) => {
            if (!res.ok) {
              throw new Error('Failed to fetch prompt')
            }
            return res.json()
          })
          .then((data) => {
            if (data.success && data.data) {
              setPromptData(data.data)
            } else {
              setError(data.message || 'No prompt data available')
            }
            setLoading(false)
          })
          .catch((err) => {
            setError(err.message || 'Failed to load prompt')
            setLoading(false)
          })
      }
    }
  }, [open, log])

  const isTruncated = (promptData?.prompt_text?.length ?? 0) >= 64000

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className='max-w-4xl max-h-[80vh] flex flex-col'>
        <DialogHeader>
          <DialogTitle>{t('Prompt Content')}</DialogTitle>
          <DialogDescription>
            {t('User ID')}: {log.user_id} • {t('Token')}: {log.token_name} •{' '}
            {t('Time')}: {formatTimestampToDate(log.created_at)}
          </DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 overflow-hidden'>
          {loading ? (
            <div className='flex items-center justify-center py-12'>
              <div className='animate-spin rounded-full h-8 w-8 border-b-2 border-primary' />
            </div>
          ) : error ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Error')}</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          ) : (
            <div className='min-h-0 h-full space-y-4 flex flex-col'>
              <ScrollArea className='min-h-0 flex-1'>
                <div className='space-y-4'>
                  <div className='bg-muted/50 rounded-md border p-4 relative'>
                    <div className='text-xs font-semibold text-muted-foreground mb-2'>{t('Prompt')}</div>
                    <Button
                      variant='ghost'
                      size='sm'
                      className='absolute top-2 right-2 h-8 w-8 p-0'
                      onClick={() =>
                        copyToClipboard(promptData?.prompt_text || '')
                      }
                      title={t('Copy to clipboard')}
                    >
                      {copiedText === promptData?.prompt_text ? (
                        <Check className='size-4 text-green-600' />
                      ) : (
                        <Copy className='size-4' />
                      )}
                    </Button>
                    <pre className='pr-10 whitespace-pre-wrap break-words text-sm font-mono leading-relaxed'>
                      {promptData?.prompt_text || t('No prompt content')}
                    </pre>
                  </div>

                  {promptData?.request_body && (
                    <div className='bg-muted/50 rounded-md border p-4 relative'>
                      <div className='text-xs font-semibold text-muted-foreground mb-2'>{t('Request Body')}</div>
                      <Button
                        variant='ghost'
                        size='sm'
                        className='absolute top-2 right-2 h-8 w-8 p-0'
                        onClick={() => copyToClipboard(promptData.request_body || '')}
                        title={t('Copy to clipboard')}
                      >
                        {copiedText === promptData.request_body ? (
                          <Check className='size-4 text-green-600' />
                        ) : (
                          <Copy className='size-4' />
                        )}
                      </Button>
                      <pre className='pr-10 whitespace-pre-wrap break-words text-sm font-mono leading-relaxed'>
                        {promptData.request_body}
                      </pre>
                    </div>
                  )}

                  {promptData?.response_body && (
                    <div className='bg-muted/50 rounded-md border p-4 relative'>
                      <div className='text-xs font-semibold text-muted-foreground mb-2'>{t('Response Body')}</div>
                      <Button
                        variant='ghost'
                        size='sm'
                        className='absolute top-2 right-2 h-8 w-8 p-0'
                        onClick={() => copyToClipboard(promptData.response_body || '')}
                        title={t('Copy to clipboard')}
                      >
                        {copiedText === promptData.response_body ? (
                          <Check className='size-4 text-green-600' />
                        ) : (
                          <Copy className='size-4' />
                        )}
                      </Button>
                      <pre className='pr-10 whitespace-pre-wrap break-words text-sm font-mono leading-relaxed'>
                        {promptData.response_body}
                      </pre>
                    </div>
                  )}
                </div>
              </ScrollArea>

              {isTruncated && (
                <Alert variant='destructive'>
                  <AlertTitle>{t('Prompt Truncated')}</AlertTitle>
                  <AlertDescription>
                    {t('This prompt exceeds 64KB limit and has been truncated')}
                  </AlertDescription>
                </Alert>
              )}
            </div>
          )}
        </div>

        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
