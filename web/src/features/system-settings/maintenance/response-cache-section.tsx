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
import { zodResolver } from '@hookform/resolvers/zod'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { MultiSelect } from '@/components/multi-select'
import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
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
import { Progress } from '@/components/ui/progress'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

/**
 * IMPORTANT: react-hook-form 7 interprets dotted `name` strings as nested
 * paths, so the schema is modelled with a nested object and only flattened
 * back to the server-side key format right before persisting. Declaring the
 * schema with literal flat keys makes saves silently turn into no-ops — see
 * the same note in performance-section.tsx.
 */
const responseCacheSchema = z.object({
  response_cache_setting: z.object({
    enabled: z.boolean(),
    share_scope: z.enum(['user', 'group', 'global']),
    enabled_models: z.array(z.string()),
    ttl_seconds: z.coerce.number().min(1),
    max_entries: z.coerce.number().min(1),
    max_request_bytes: z.coerce.number().min(1024),
    max_response_bytes: z.coerce.number().min(1024),
    max_temperature: z.coerce.number().min(0).max(2),
    cache_tool_requests: z.boolean(),
    hit_billing: z.enum(['free', 'ratio']),
    hit_billing_ratio: z.coerce.number().min(0).max(1),
    stream_replay_chunk_size: z.coerce.number().min(1),
  }),
})

type ResponseCacheFormInput = z.input<typeof responseCacheSchema>
type ResponseCacheFormValues = z.output<typeof responseCacheSchema>

export type FlatResponseCacheDefaults = {
  'response_cache_setting.enabled': boolean
  'response_cache_setting.share_scope': string
  'response_cache_setting.enabled_models': string[]
  'response_cache_setting.ttl_seconds': number
  'response_cache_setting.max_entries': number
  'response_cache_setting.max_request_bytes': number
  'response_cache_setting.max_response_bytes': number
  'response_cache_setting.max_temperature': number
  'response_cache_setting.cache_tool_requests': boolean
  'response_cache_setting.hit_billing': string
  'response_cache_setting.hit_billing_ratio': number
  'response_cache_setting.stream_replay_chunk_size': number
}

const SHARE_SCOPES = ['user', 'group', 'global'] as const
const HIT_BILLING_MODES = ['free', 'ratio'] as const

type ShareScope = (typeof SHARE_SCOPES)[number]
type HitBilling = (typeof HIT_BILLING_MODES)[number]

// The backend normalises unknown values to the conservative option; mirror that
// here so a hand-edited option row cannot render an empty select.
const toShareScope = (value: string): ShareScope =>
  (SHARE_SCOPES as readonly string[]).includes(value)
    ? (value as ShareScope)
    : 'user'

const toHitBilling = (value: string): HitBilling =>
  (HIT_BILLING_MODES as readonly string[]).includes(value)
    ? (value as HitBilling)
    : 'free'

const buildFormDefaults = (
  defaults: FlatResponseCacheDefaults
): ResponseCacheFormInput => ({
  response_cache_setting: {
    enabled: defaults['response_cache_setting.enabled'],
    share_scope: toShareScope(defaults['response_cache_setting.share_scope']),
    enabled_models: defaults['response_cache_setting.enabled_models'] ?? [],
    ttl_seconds: defaults['response_cache_setting.ttl_seconds'],
    max_entries: defaults['response_cache_setting.max_entries'],
    max_request_bytes: defaults['response_cache_setting.max_request_bytes'],
    max_response_bytes: defaults['response_cache_setting.max_response_bytes'],
    max_temperature: defaults['response_cache_setting.max_temperature'],
    cache_tool_requests: defaults['response_cache_setting.cache_tool_requests'],
    hit_billing: toHitBilling(defaults['response_cache_setting.hit_billing']),
    hit_billing_ratio: defaults['response_cache_setting.hit_billing_ratio'],
    stream_replay_chunk_size:
      defaults['response_cache_setting.stream_replay_chunk_size'],
  },
})

const normalizeFormValues = (
  values: ResponseCacheFormValues
): FlatResponseCacheDefaults => ({
  'response_cache_setting.enabled': values.response_cache_setting.enabled,
  'response_cache_setting.share_scope':
    values.response_cache_setting.share_scope,
  'response_cache_setting.enabled_models':
    values.response_cache_setting.enabled_models,
  'response_cache_setting.ttl_seconds':
    values.response_cache_setting.ttl_seconds,
  'response_cache_setting.max_entries':
    values.response_cache_setting.max_entries,
  'response_cache_setting.max_request_bytes':
    values.response_cache_setting.max_request_bytes,
  'response_cache_setting.max_response_bytes':
    values.response_cache_setting.max_response_bytes,
  'response_cache_setting.max_temperature':
    values.response_cache_setting.max_temperature,
  'response_cache_setting.cache_tool_requests':
    values.response_cache_setting.cache_tool_requests,
  'response_cache_setting.hit_billing':
    values.response_cache_setting.hit_billing,
  'response_cache_setting.hit_billing_ratio':
    values.response_cache_setting.hit_billing_ratio,
  'response_cache_setting.stream_replay_chunk_size':
    values.response_cache_setting.stream_replay_chunk_size,
})

type ResponseCacheStats = {
  enabled: boolean
  share_scope: string
  enabled_models: number
  hits: number
  misses: number
  stores: number
  hit_rate: number
  entries: number
  cache_algo: string
}

interface Props {
  defaultValues: FlatResponseCacheDefaults
}

export function ResponseCacheSection(props: Props) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [stats, setStats] = useState<ResponseCacheStats | null>(null)
  const [modelOptions, setModelOptions] = useState<string[]>([])

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<
    ResponseCacheFormInput,
    unknown,
    ResponseCacheFormValues
  >({
    resolver: zodResolver(responseCacheSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatResponseCacheDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const fetchStats = useCallback(async () => {
    try {
      const res = await api.get('/api/option/response_cache')
      if (res.data.success) setStats(res.data.data)
    } catch {
      /* stats are advisory; a failure must not block the settings form */
    }
  }, [])

  useEffect(() => {
    fetchStats()
  }, [fetchStats])

  // Model names are only suggestions — the whitelist accepts free-form entries
  // so an admin can whitelist a deployment the pricing table does not list yet.
  useEffect(() => {
    let cancelled = false
    api
      .get('/api/models/')
      .then((res) => {
        if (cancelled || !res.data.success) return
        const raw = res.data.data
        const names = Array.isArray(raw)
          ? raw
              .map((item: unknown) =>
                typeof item === 'string'
                  ? item
                  : ((item as { model_name?: string })?.model_name ?? '')
              )
              .filter(Boolean)
          : []
        setModelOptions([...new Set(names as string[])].sort())
      })
      .catch(() => {
        /* suggestions are optional; free-form entry still works */
      })
    return () => {
      cancelled = true
    }
  }, [])

  const onSubmit = async (values: ResponseCacheFormValues) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatResponseCacheDefaults>
    ).filter(
      (key) =>
        JSON.stringify(normalized[key]) !==
        JSON.stringify(baselineRef.current[key])
    )

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      const value = normalized[key]
      await updateOption.mutateAsync({
        key,
        // Options are stored as strings and the backend config manager
        // JSON-decodes slice-typed fields, so the whitelist goes over as JSON.
        value: Array.isArray(value) ? JSON.stringify(value) : value,
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
    fetchStats()
  }

  const clearCache = async () => {
    try {
      const res = await api.delete('/api/option/response_cache')
      if (res.data.success) {
        toast.success(t('Response cache cleared'))
        fetchStats()
      }
    } catch {
      toast.error(t('Cleanup failed'))
    }
  }

  const enabled = form.watch('response_cache_setting.enabled')
  const shareScope = form.watch('response_cache_setting.share_scope')
  const hitBilling = form.watch('response_cache_setting.hit_billing')
  const enabledModels = form.watch('response_cache_setting.enabled_models')

  const hitRatePercent = stats ? Math.round(stats.hit_rate * 1000) / 10 : 0
  const totalLookups = stats ? stats.hits + stats.misses : 0
  const sharesAcrossUsers = shareScope === 'group' || shareScope === 'global'
  const whitelistEmpty = enabled && (enabledModels?.length ?? 0) === 0

  return (
    <SettingsSection title={t('Gateway Response Cache')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <div className='col-span-full space-y-1'>
            <p className='text-muted-foreground text-sm'>
              {t(
                'Serves a repeated request straight from the gateway without calling the upstream channel. Exact match only — requests must be byte-identical apart from fields that cannot affect the output.'
              )}
            </p>
          </div>

          {/* Hit rate is the only evidence for whether this feature earns its
              keep, so it leads the section instead of hiding below the form. */}
          <div className='col-span-full rounded-lg border p-4'>
            <div className='grid grid-cols-2 gap-4 md:grid-cols-4'>
              <div>
                <p className='text-muted-foreground text-xs'>{t('Hit Rate')}</p>
                <p className='text-2xl font-semibold'>
                  {totalLookups > 0 ? `${hitRatePercent}%` : '—'}
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>{t('Entries')}</p>
                <p className='text-2xl font-semibold'>{stats?.entries ?? 0}</p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>
                  {t('Hits / Misses')}
                </p>
                <p className='text-2xl font-semibold'>
                  {stats?.hits ?? 0}
                  <span className='text-muted-foreground text-base'>
                    {' / '}
                    {stats?.misses ?? 0}
                  </span>
                </p>
              </div>
              <div>
                <p className='text-muted-foreground text-xs'>{t('Stores')}</p>
                <p className='text-2xl font-semibold'>{stats?.stores ?? 0}</p>
              </div>
            </div>

            {totalLookups > 0 && (
              <Progress value={hitRatePercent} className='mt-3 h-2' />
            )}

            <div className='mt-3 flex flex-wrap items-center justify-between gap-2'>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Counters are per-process and reset on restart. Backend: {{algo}}.',
                  { algo: stats?.cache_algo || '—' }
                )}
              </p>
              <div className='flex gap-2'>
                <Button
                  type='button'
                  size='sm'
                  variant='outline'
                  onClick={fetchStats}
                >
                  {t('Refresh')}
                </Button>
                <AlertDialog>
                  <AlertDialogTrigger
                    render={
                      <Button type='button' size='sm' variant='outline'>
                        {t('Clear Cache')}
                      </Button>
                    }
                  />
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        {t('Clear the response cache?')}
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        {t(
                          'Every stored entry is dropped and the hit counters reset. Subsequent requests go upstream until the cache refills.'
                        )}
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
                      <AlertDialogAction onClick={clearCache}>
                        {t('Clear Cache')}
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </div>
            </div>
          </div>

          <FormField
            control={form.control}
            name='response_cache_setting.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Response Cache')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Takes effect only for models listed in the whitelist below.'
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
            name='response_cache_setting.enabled_models'
            render={({ field }) => (
              <FormItem data-settings-form-span='full'>
                <FormLabel>{t('Cached Models')}</FormLabel>
                <FormControl>
                  <MultiSelect
                    options={modelOptions.map((name) => ({
                      value: name,
                      label: name,
                    }))}
                    selected={field.value ?? []}
                    onChange={field.onChange}
                    allowCreate
                    disabled={!enabled}
                    placeholder={t('Select or type a model name...')}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'An empty list keeps the feature inert. Whether caching is safe depends on what a model is used for, so each one must be opted in explicitly.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {whitelistEmpty && (
            <Alert>
              <AlertDescription>
                {t(
                  'The cache is enabled but no model is whitelisted, so nothing will be cached.'
                )}
              </AlertDescription>
            </Alert>
          )}

          <FormField
            control={form.control}
            name='response_cache_setting.share_scope'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Share Scope')}</FormLabel>
                <Select
                  items={[
                    { value: 'user', label: t('Per user') },
                    { value: 'group', label: t('Per group') },
                    { value: 'global', label: t('Global') },
                  ]}
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='user'>{t('Per user')}</SelectItem>
                      <SelectItem value='group'>{t('Per group')}</SelectItem>
                      <SelectItem value='global'>{t('Global')}</SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t('Decides who can be served the same cached response.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.ttl_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Entry TTL (seconds)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Cached entries hold user conversation content; there is deliberately no never-expire option.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          {/* Shown the moment the scope changes rather than on save, so the
              consequence is visible while the choice is being made. */}
          {sharesAcrossUsers && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t(
                  'This scope shares cached entries between users, meaning one user can be served another user’s model output. Use it only for a single application or a trusted team.'
                )}
              </AlertDescription>
            </Alert>
          )}

          <Separator className='col-span-full' />

          <div className='col-span-full'>
            <h4 className='text-sm font-medium'>{t('Advanced')}</h4>
          </div>

          <FormField
            control={form.control}
            name='response_cache_setting.max_temperature'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maximum Temperature')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={2}
                    step={0.1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Requests above this temperature are never cached. Requests that omit temperature are treated as cacheable.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.max_entries'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maximum Entries')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Applies only to the in-process cache used when Redis is disabled.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.max_request_bytes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maximum Request Size (bytes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1024}
                    step={1024}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.max_response_bytes'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Maximum Response Size (bytes)')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1024}
                    step={1024}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.stream_replay_chunk_size'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Stream Replay Chunk Size')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    step={1}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled}
                  />
                </FormControl>
                <FormDescription>
                  {t('Characters per delta when replaying a cached stream.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.hit_billing'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Billing on Hit')}</FormLabel>
                <Select
                  items={[
                    { value: 'free', label: t('No charge') },
                    { value: 'ratio', label: t('Fraction of full price') },
                  ]}
                  value={field.value}
                  onValueChange={field.onChange}
                  disabled={!enabled}
                >
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent alignItemWithTrigger={false}>
                    <SelectGroup>
                      <SelectItem value='free'>{t('No charge')}</SelectItem>
                      <SelectItem value='ratio'>
                        {t('Fraction of full price')}
                      </SelectItem>
                    </SelectGroup>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Free suits a self-hosted gateway where the operator pays for the tokens. When reselling, a hit charged at zero gives the request away and loses the margin.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.hit_billing_ratio'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Hit Price Ratio')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    max={1}
                    step={0.05}
                    {...safeNumberFieldProps(field)}
                    disabled={!enabled || hitBilling !== 'ratio'}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Fraction of the normal price charged on a hit. Values outside (0, 1] fall back to full price.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='response_cache_setting.cache_tool_requests'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Cache Requests With Tools')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Off by default: tool calls usually sit mid-agent-loop, where reusing a historical tool_call_id corrupts client state. Responses containing tool calls are never stored regardless.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!enabled}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            onReset={() => form.reset(formDefaults)}
            isResetDisabled={!form.formState.isDirty}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
