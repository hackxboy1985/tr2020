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
import { useMemo } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { parseHttpStatusCodeRules } from '@/lib/http-status-code-rules'
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
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

/**
 * react-hook-form treats dotted `name` strings as nested paths, so the
 * overload sleep options are modelled as a nested object here and flattened
 * back to their server-side keys (`overload_sleep_setting.*`) on submit.
 */
type FlatBehaviorDefaults = {
  RetryTimes: number
  DefaultCollapseSidebar: boolean
  DemoSiteEnabled: boolean
  SelfUseModeEnabled: boolean
  'overload_sleep_setting.enabled': boolean
  'overload_sleep_setting.sleep_minutes': number
  'overload_sleep_setting.status_codes': string
  'overload_sleep_setting.fallback_models': string
}

const behaviorSchema = z
  .object({
    RetryTimes: z.coerce.number().min(0).max(10),
    DefaultCollapseSidebar: z.boolean(),
    DemoSiteEnabled: z.boolean(),
    SelfUseModeEnabled: z.boolean(),
    overload_sleep_setting: z.object({
      enabled: z.boolean(),
      sleep_minutes: z.coerce.number().int().min(1).max(1440),
      status_codes: z.string(),
      // Free-form model mapping, one "origin=fallback" pair per line; empty is allowed.
      fallback_models: z.string(),
    }),
  })
  .superRefine((values, ctx) => {
    const parsed = parseHttpStatusCodeRules(
      values.overload_sleep_setting.status_codes
    )
    if (!parsed.ok) {
      ctx.addIssue({
        code: 'custom',
        path: ['overload_sleep_setting', 'status_codes'],
        message: `Invalid status code rules: ${parsed.invalidTokens.join(', ')}`,
      })
    }
  })

type BehaviorFormInput = z.input<typeof behaviorSchema>
type BehaviorFormValues = z.output<typeof behaviorSchema>

type SystemBehaviorSectionProps = {
  defaultValues: FlatBehaviorDefaults
}

const buildFormDefaults = (
  defaults: FlatBehaviorDefaults
): BehaviorFormInput => ({
  RetryTimes: defaults.RetryTimes,
  DefaultCollapseSidebar: defaults.DefaultCollapseSidebar,
  DemoSiteEnabled: defaults.DemoSiteEnabled,
  SelfUseModeEnabled: defaults.SelfUseModeEnabled,
  overload_sleep_setting: {
    enabled: defaults['overload_sleep_setting.enabled'] ?? false,
    sleep_minutes: defaults['overload_sleep_setting.sleep_minutes'] ?? 2,
    status_codes: defaults['overload_sleep_setting.status_codes'] ?? '429,503',
    fallback_models: defaults['overload_sleep_setting.fallback_models'] ?? '',
  },
})

const flattenFormValues = (
  values: BehaviorFormValues
): FlatBehaviorDefaults => ({
  RetryTimes: values.RetryTimes,
  DefaultCollapseSidebar: values.DefaultCollapseSidebar,
  DemoSiteEnabled: values.DemoSiteEnabled,
  SelfUseModeEnabled: values.SelfUseModeEnabled,
  'overload_sleep_setting.enabled': values.overload_sleep_setting.enabled,
  'overload_sleep_setting.sleep_minutes':
    values.overload_sleep_setting.sleep_minutes,
  'overload_sleep_setting.status_codes':
    values.overload_sleep_setting.status_codes,
  'overload_sleep_setting.fallback_models':
    values.overload_sleep_setting.fallback_models,
})

export function SystemBehaviorSection(props: SystemBehaviorSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<BehaviorFormInput, unknown, BehaviorFormValues>({
    resolver: zodResolver(behaviorSchema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const onSubmit = async (values: BehaviorFormValues) => {
    const normalized = flattenFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatBehaviorDefaults>
    ).filter((key) => normalized[key] !== props.defaultValues[key])

    for (const key of changedKeys) {
      await updateOption.mutateAsync({ key, value: normalized[key] })
    }
  }

  const sleepEnabled = form.watch('overload_sleep_setting.enabled')

  return (
    <SettingsSection title={t('System Behavior')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='RetryTimes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Retry Times')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='0'
                    max='10'
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Number of times to retry failed requests (0-10)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='DefaultCollapseSidebar'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Default Collapse Sidebar')}</FormLabel>
                  <FormDescription>
                    {t('Sidebar collapsed by default for new users')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='DemoSiteEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Demo Site Mode')}</FormLabel>
                  <FormDescription>
                    {t('Enable demo mode with limited functionality')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='SelfUseModeEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Self-Use Mode')}</FormLabel>
                  <FormDescription>
                    {t('Optimize system for self-hosted single-user usage')}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <Separator className='lg:col-span-2' />

          <div data-settings-form-span='full'>
            <h4 className='font-medium'>{t('Overload Sleep Pool')}</h4>
            <p className='text-muted-foreground mt-1 text-xs'>
              {t(
                'When a channel returns one of the configured overload status codes, it is put to sleep for the configured duration. Later requests skip sleeping channels for the same model.'
              )}
            </p>
          </div>

          <FormField
            control={form.control}
            name='overload_sleep_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Overload Sleep Pool')}</FormLabel>
                  <FormDescription>
                    {t(
                      'When disabled, no channels are put to sleep and no requests are skipped.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='overload_sleep_setting.sleep_minutes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Sleep Duration (minutes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min='1'
                    max='1440'
                    step='1'
                    {...safeNumberFieldProps(field)}
                    disabled={!sleepEnabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'How long a channel stays asleep after an overload response (1-1440 minutes, default 2).'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='overload_sleep_setting.status_codes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Overload Status Codes')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='429,503'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                    disabled={!sleepEnabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma-separated status codes; ranges are supported (for example 429,500-599).'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='overload_sleep_setting.fallback_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Fallback Models')}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder='gemini-3.6-flash=gemini-3.7-flash'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                    rows={4}
                    disabled={!sleepEnabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'One mapping per line, format: origin-model=fallback-model. Only used when all channels of the origin model are sleeping AND the API key enables "Use Similar Model Fallback".'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
