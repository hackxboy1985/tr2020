import { useEffect } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Switch } from '@/components/ui/switch'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const videoGenerationSchema = z.object({
  VideoGenerationEnabled: z.boolean(),
  VideoGenerationChannel: z.enum(['platform', 'coze']),
  VideoGenerationPlatformBaseURL: z.string(),
  VideoGenerationPlatformApiKey: z.string(),
  VideoGenerationPlatformApiSecret: z.string(),
  VideoGenerationCozeApiKey: z.string(),
  VideoGenerationCozeWorkflowId: z.string(),
  VideoGenerationCozeWebhookSecret: z.string(),
  VideoGenerationCozeBaseURL: z.string(),
})

type VideoGenerationFormValues = z.infer<typeof videoGenerationSchema>

type VideoGenerationSettingsSectionProps = {
  defaultValues: VideoGenerationFormValues
}

export function VideoGenerationSettingsSection({
  defaultValues,
}: VideoGenerationSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<VideoGenerationFormValues>({
    resolver: zodResolver(videoGenerationSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const channel = form.watch('VideoGenerationChannel')

  const onSubmit = async (values: VideoGenerationFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof VideoGenerationFormValues]
    )
    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value })
    }
  }

  return (
    <SettingsSection title={t('Video Generation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save video generation settings'
          />
          <div className='space-y-6'>
            {/* 启用开关 */}
            <FormField
              control={form.control}
              name='VideoGenerationEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable video generation')}</FormLabel>
                    <FormDescription>
                      {t('Allow users to create AI video generation projects.')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                  <FormMessage />
                </SettingsSwitchItem>
              )}
            />

            {/* 渠道选择 */}
            <FormField
              control={form.control}
              name='VideoGenerationChannel'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Channel')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Select the upstream channel for video generation. "Platform" uses a third-party service that wraps Coze; "Coze" connects directly to the Coze workflow API.'
                    )}
                  </FormDescription>
                  <Select
                    onValueChange={field.onChange}
                    defaultValue={field.value}
                    value={field.value}
                  >
                    <FormControl>
                      <SelectTrigger className='w-48'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='platform'>
                        {t('Third-party Platform')}
                      </SelectItem>
                      <SelectItem value='coze'>{t('Coze')}</SelectItem>
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Platform 渠道配置 */}
            {channel === 'platform' && (
              <div className='space-y-4 rounded-lg border p-4'>
                <p className='text-sm font-medium'>{t('Platform Settings')}</p>
                <FormField
                  control={form.control}
                  name='VideoGenerationPlatformBaseURL'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Base URL')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder='https://your-platform.example.com'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='VideoGenerationPlatformApiKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('API Key')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Enter API key')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='VideoGenerationPlatformApiSecret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('Webhook Secret')}{' '}
                        <span className='text-muted-foreground font-normal'>
                          ({t('optional')})
                        </span>
                      </FormLabel>
                      <FormDescription>
                        {t('Used to verify webhook callback signatures.')}
                      </FormDescription>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Enter webhook secret')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

            {/* Coze 渠道配置 */}
            {channel === 'coze' && (
              <div className='space-y-4 rounded-lg border p-4'>
                <p className='text-sm font-medium'>{t('Coze Settings')}</p>
                <FormField
                  control={form.control}
                  name='VideoGenerationCozeApiKey'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('API Key')}</FormLabel>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Enter Coze API key')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='VideoGenerationCozeWorkflowId'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Workflow ID')}</FormLabel>
                      <FormControl>
                        <Input
                          placeholder={t('Enter Coze workflow ID')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='VideoGenerationCozeWebhookSecret'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Webhook Secret')}</FormLabel>
                      <FormDescription>
                        {t('Used to verify Coze webhook callback signatures.')}
                      </FormDescription>
                      <FormControl>
                        <Input
                          type='password'
                          placeholder={t('Enter webhook secret')}
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name='VideoGenerationCozeBaseURL'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>
                        {t('Base URL')}{' '}
                        <span className='text-muted-foreground font-normal'>
                          ({t('optional')})
                        </span>
                      </FormLabel>
                      <FormDescription>
                        {t('Default: https://api.coze.cn')}
                      </FormDescription>
                      <FormControl>
                        <Input
                          placeholder='https://api.coze.cn'
                          {...field}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            )}

            {/* Webhook 回调地址提示 */}
            <div className='rounded-lg bg-muted p-4 text-sm space-y-1'>
              <p className='font-medium'>{t('Webhook Callback URLs')}</p>
              <p className='text-muted-foreground'>
                {t('Configure these URLs in your upstream platform')}:
              </p>
              <p className='font-mono text-xs break-all'>
                {window.location.origin}/api/video-generation/webhook/
                {channel}
              </p>
            </div>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
