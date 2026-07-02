import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetFooter,
} from '@/components/ui/sheet'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { createVideoChannel, updateVideoChannel } from '../api'
import { videoChannelFormSchema, type VideoChannel, type VideoChannelFormValues } from '../types'
import { useVideoChannels } from './video-channels-provider'

interface VideoChannelMutateDrawerProps {
  open: boolean
  onOpenChange: (v: boolean) => void
  currentRow: VideoChannel | null
}

const defaultValues: VideoChannelFormValues = {
  name: '',
  channel_type: 'platform',
  base_url: '',
  api_key: '',
  api_secret: '',
  workflow_id: '',
  create_path: '',
  status_query_path: '',
  groups: 'default',
  weight: 1,
  enabled: 1,
  remark: '',
}

export function VideoChannelMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: VideoChannelMutateDrawerProps) {
  const { t } = useTranslation()
  const { refresh } = useVideoChannels()
  const isEdit = !!currentRow

  const form = useForm<VideoChannelFormValues>({
    resolver: zodResolver(videoChannelFormSchema),
    defaultValues,
  })

  const channelType = form.watch('channel_type')

  useEffect(() => {
    if (open) {
      if (currentRow) {
        form.reset({
          name: currentRow.name,
          channel_type: currentRow.channel_type,
          base_url: currentRow.base_url,
          api_key: '',   // 不回填密钥
          api_secret: '',
          workflow_id: currentRow.workflow_id,
          create_path: currentRow.create_path,
          status_query_path: currentRow.status_query_path,
          groups: currentRow.groups,
          weight: currentRow.weight,
          enabled: currentRow.enabled,
          remark: currentRow.remark,
        })
      } else {
        form.reset(defaultValues)
      }
    }
  }, [open, currentRow, form])

  const mutation = useMutation({
    mutationFn: (values: VideoChannelFormValues) =>
      isEdit
        ? updateVideoChannel(currentRow!.id, values)
        : createVideoChannel(values),
    onSuccess: () => {
      refresh()
      onOpenChange(false)
    },
  })

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className='overflow-y-auto sm:max-w-lg'>
        <SheetHeader>
          <SheetTitle>
            {isEdit ? t('Edit Video Channel') : t('Add Video Channel')}
          </SheetTitle>
        </SheetHeader>

        <Form {...form}>
          <form
            onSubmit={form.handleSubmit((v) => mutation.mutate(v))}
            className='space-y-4 py-4'
          >
            {/* 名称 */}
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('e.g. Coze-A')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* 渠道类型 */}
            <FormField
              control={form.control}
              name='channel_type'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel Type')}</FormLabel>
                  <Select
                    onValueChange={field.onChange}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='platform'>{t('Third-party Platform')}</SelectItem>
                      <SelectItem value='coze'>Coze</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Base URL */}
            <FormField
              control={form.control}
              name='base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={
                        channelType === 'coze'
                          ? 'https://api.coze.cn'
                          : 'https://your-platform.example.com'
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* API Key */}
            <FormField
              control={form.control}
              name='api_key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('API Key')}
                    {isEdit && (
                      <span className='text-muted-foreground ml-2 text-xs'>
                        ({t('leave blank to keep existing')})
                      </span>
                    )}
                  </FormLabel>
                  <FormControl>
                    <Input type='password' placeholder={t('Enter API key')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Workflow ID（Coze 渠道） */}
            {channelType === 'coze' && (
              <FormField
                control={form.control}
                name='workflow_id'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Workflow ID')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('Enter Coze workflow ID')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            )}

            {/* API Secret / Webhook Secret */}
            <FormField
              control={form.control}
              name='api_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Webhook Secret')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('optional')})
                    </span>
                  </FormLabel>
                  <FormDescription>
                    {t('Used to verify webhook callback signatures.')}
                  </FormDescription>
                  <FormControl>
                    <Input type='password' placeholder={t('Enter webhook secret')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Create Path */}
            <FormField
              control={form.control}
              name='create_path'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Create Path')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('optional')})
                    </span>
                  </FormLabel>
                  <FormDescription>
                    {t('Default: {{path}}', {
                      path: channelType === 'coze' ? '/v1/workflow/run' : '/api/video/create',
                    })}
                  </FormDescription>
                  <FormControl>
                    <Input
                      placeholder={
                        channelType === 'coze' ? '/v1/workflow/run' : '/api/video/create'
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Status Query Path */}
            <FormField
              control={form.control}
              name='status_query_path'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Status Query Path')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('optional')})
                    </span>
                  </FormLabel>
                  <FormDescription>
                    {t('Use {id} as placeholder. Default: {{path}}', {
                      path:
                        channelType === 'coze'
                          ? '/v1/workflow/run/{id}'
                          : '/api/video/projects/{id}',
                    })}
                  </FormDescription>
                  <FormControl>
                    <Input
                      placeholder={
                        channelType === 'coze'
                          ? '/v1/workflow/run/{id}'
                          : '/api/video/projects/{id}'
                      }
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Groups */}
            <FormField
              control={form.control}
              name='groups'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Groups')}</FormLabel>
                  <FormDescription>
                    {t('Comma-separated group names, e.g. default,vip')}
                  </FormDescription>
                  <FormControl>
                    <Input placeholder='default' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Weight */}
            <FormField
              control={form.control}
              name='weight'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Weight')}</FormLabel>
                  <FormDescription>
                    {t('Higher weight = more traffic. Used for weighted random selection within the same group.')}
                  </FormDescription>
                  <FormControl>
                    <Input type='number' min={1} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Remark */}
            <FormField
              control={form.control}
              name='remark'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Remark')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('optional')})
                    </span>
                  </FormLabel>
                  <FormControl>
                    <Input placeholder={t('Internal note')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Webhook URL hint */}
            <div className='bg-muted rounded-lg p-3 text-xs space-y-1'>
              <p className='font-medium'>{t('Webhook Callback URL')}</p>
              <p className='text-muted-foreground break-all font-mono'>
                {window.location.origin}/api/video-generation/webhook/
                <span className='text-foreground'>{'{channel_id}'}</span>
              </p>
              <p className='text-muted-foreground'>
                {t('Replace {channel_id} with this channel\'s ID after creation.')}
              </p>
            </div>

            <SheetFooter className='pt-4'>
              <Button
                variant='outline'
                type='button'
                onClick={() => onOpenChange(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={mutation.isPending}>
                {mutation.isPending ? t('Saving...') : t('Save')}
              </Button>
            </SheetFooter>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
