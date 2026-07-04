import { useEffect, useState, useMemo } from 'react'
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
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Button } from '@/components/ui/button'
import { Loader2, CheckCircle2, XCircle } from 'lucide-react'
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
  save_request_response: 0,
  model_mapping: '',
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

  type TestStatus = 'idle' | 'testing' | 'ok' | 'fail'
  const [testStatus, setTestStatus] = useState<TestStatus>('idle')

  const testConnection = async () => {
    const url = form.getValues('base_url')
    if (!url) return
    setTestStatus('testing')
    try {
      const resp = await fetch(url, { method: 'HEAD', mode: 'no-cors' })
      // no-cors 下 type=opaque，无法读取状态码，但不抛错即认为可达
      void resp
      setTestStatus('ok')
    } catch {
      setTestStatus('fail')
    }
  }

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
    rate_limit: currentRow.rate_limit ?? 1,
    pre_deduct_quota: currentRow.pre_deduct_quota ?? 0,
    price_per_second: currentRow.price_per_second ?? 1,
          save_request_response: currentRow.save_request_response ?? 0,
          model_mapping: currentRow.model_mapping ?? '',
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
                  <div className='flex gap-2'>
                    <FormControl>
                      <Input
                        placeholder={
                          channelType === 'coze'
                            ? 'https://api.coze.cn'
                            : 'https://your-platform.example.com'
                        }
                        {...field}
                        onChange={(e) => {
                          field.onChange(e)
                          setTestStatus('idle')
                        }}
                      />
                    </FormControl>
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='shrink-0'
                      disabled={!form.watch('base_url') || testStatus === 'testing'}
                      onClick={testConnection}
                    >
                      {testStatus === 'testing' && <Loader2 className='h-4 w-4 animate-spin' />}
                      {testStatus === 'ok' && <CheckCircle2 className='h-4 w-4 text-green-500' />}
                      {testStatus === 'fail' && <XCircle className='h-4 w-4 text-destructive' />}
                      {testStatus === 'idle' && t('Test')}
                    </Button>
                  </div>
                  {testStatus === 'ok' && (
                    <p className='text-xs text-green-600'>{t('Connection successful')}</p>
                  )}
                  {testStatus === 'fail' && (
                    <p className='text-xs text-destructive'>{t('Connection failed or unreachable')}</p>
                  )}
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


            {/* Price Per Second */}
            <FormField
              control={form.control}
              name='price_per_second'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Price Per Second (quota)')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('default 1 ≈ 1 yuan')})
                    </span>
                  </FormLabel>
                  <FormControl>
                    <Input type='number' placeholder='1' {...field} onChange={e => field.onChange(Number(e.target.value))} />
                  </FormControl>
                  <FormDescription>{t('Default price when model has no specific price in Model Config. Pre-deduction = duration × price')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Rate Limit */}
            <FormField
              control={form.control}
              name='rate_limit'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('QPS Limit')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('0 = unlimited')})
                    </span>
                  </FormLabel>
                  <FormControl>
                    <Input type='number' placeholder='1' {...field} onChange={e => field.onChange(Number(e.target.value))} />
                  </FormControl>
                  <FormDescription>{t('Max requests per second per API key')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Pre-deduct Quota */}
            <FormField
              control={form.control}
              name='pre_deduct_quota'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Pre-deduct Quota')}
                    <span className='text-muted-foreground ml-2 text-xs'>
                      ({t('0 = auto from price × duration')})
                    </span>
                  </FormLabel>
                  <FormControl>
                    <Input type='number' placeholder='0' {...field} onChange={e => field.onChange(Number(e.target.value))} />
                  </FormControl>
                  <FormDescription>{t('Fixed pre-deduction amount, overrides price calculation')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {/* Model Config */}
            <ModelConfigTable
              mapping={form.watch('model_mapping')}
              prices={form.watch('model_prices')}
              onMappingChange={(v) => form.setValue('model_mapping', v)}
              onPricesChange={(v) => form.setValue('model_prices', v)}
            />

            {/* Model Config (3-column: name | upstream | price) */}
            <ModelConfigTable
              mapping={form.watch('model_mapping')}
              prices={form.watch('model_prices')}
              onMappingChange={(v) => form.setValue('model_mapping', v)}
              onPricesChange={(v) => form.setValue('model_prices', v)}
            />

            {/* Save Request/Response */}
            <FormField
              control={form.control}
              name='save_request_response'
              render={({ field }) => (
                <FormItem className='flex flex-row items-center justify-between rounded-lg border p-3'>
                  <div className='space-y-0.5'>
                    <FormLabel>{t('Save Request & Response')}</FormLabel>
                    <FormDescription>
                      {t('Save upstream request and response body (first 500 chars) alongside prompt logs.')}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value === 1}
                      onCheckedChange={(checked) => field.onChange(checked ? 1 : 0)}
                    />
                  </FormControl>
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
function ModelConfigTable({ mapping, prices, onMappingChange, onPricesChange }: {
  mapping: string
  prices: string
  onMappingChange: (v: string) => void
  onPricesChange: (v: string) => void
}) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<Array<{ model: string; upstream: string; price: number }>>([])
  const [newModel, setNewModel] = useState('')

  useEffect(() => {
    try {
      const m = JSON.parse(mapping || '{}')
      const p = JSON.parse(prices || '{}')
      const keys = Object.keys(m)
      if (keys.length === 0 && rows.length > 0) return
      setRows(keys.map(k => ({ model: k, upstream: m[k] || '', price: p[k] ?? 1 })))
    } catch {}
  }, [mapping, prices])

  const emitChanges = (nextRows: typeof rows) => {
    const m: Record<string, string> = {}
    const p: Record<string, number> = {}
    nextRows.forEach(r => {
      if (r.model.trim()) {
        m[r.model.trim()] = r.upstream
        p[r.model.trim()] = r.price
      }
    })
    onMappingChange(JSON.stringify(m, null, 2))
    onPricesChange(JSON.stringify(p, null, 2))
  }

  const addRow = () => {
    const name = newModel.trim()
    if (!name) return
    const next = [...rows, { model: name, upstream: '', price: 1 }]
    setRows(next)
    emitChanges(next)
    setNewModel('')
  }

  const removeRow = (idx: number) => {
    const next = rows.filter((_, i) => i !== idx)
    setRows(next)
    emitChanges(next)
  }

  const updateRow = (idx: number, field: string, value: string | number) => {
    const next = [...rows]
    next[idx] = { ...next[idx], [field]: value }
    setRows(next)
    emitChanges(next)
  }

  return (
    <div className='space-y-2'>
      <Label className='flex items-center gap-1.5 text-xs font-semibold'>{t('Model Config')}</Label>
      <div className='overflow-hidden rounded-md border'>
        <table className='w-full text-xs'>
          <thead>
            <tr className='bg-muted/50'>
              <th className='px-3 py-2 text-left font-medium'>{t('Model Name')}</th>
              <th className='px-3 py-2 text-left font-medium'>{t('Upstream ID')}</th>
              <th className='px-3 py-2 text-left font-medium w-20'>{t('Price/s')}<span className='text-muted-foreground ml-1'>({t('预扣计算')})</span></th>
              <th className='w-10'></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row, i) => (
              <tr key={i} className='border-t'>
                <td className='px-3 py-2'><input className='flex h-7 w-full rounded-md border border-input bg-transparent px-2 text-xs' value={row.model} onChange={e => updateRow(i, 'model', e.target.value)} /></td>
                <td className='px-3 py-2'><input className='flex h-7 w-full rounded-md border border-input bg-transparent px-2 text-xs' value={row.upstream} onChange={e => updateRow(i, 'upstream', e.target.value)} /></td>
                <td className='px-3 py-2'><input type='number' min={0} step={0.001} className='flex h-7 w-full rounded-md border border-input bg-transparent px-2 text-xs' value={row.price} onChange={e => updateRow(i, 'price', parseFloat(e.target.value) || 0)} /></td>
                <td className='px-3 py-2'><button type='button' className='text-destructive hover:text-destructive/80 text-xs' onClick={() => removeRow(i)}>&times;</button></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className='flex gap-2'>
        <input className='flex h-7 flex-1 rounded-md border border-input bg-transparent px-2 text-xs' placeholder={t('Add model...')} value={newModel} onChange={e => setNewModel(e.target.value)} onKeyDown={e => e.key === 'Enter' && addRow()} />
        <button type='button' className='bg-primary text-primary-foreground hover:bg-primary/80 rounded-md px-3 text-xs h-7' onClick={addRow}>{t('Add')}</button>
      </div>
      <p className='text-muted-foreground text-xs'>{t('Each row: model name → upstream ID mapping + per-second price.')}</p>
    </div>
  )
}