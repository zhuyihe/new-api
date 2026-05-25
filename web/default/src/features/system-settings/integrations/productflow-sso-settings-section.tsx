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
import { useEffect, useMemo, useRef, useState } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { AlertTriangle, Info } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import {
  useChannelGroups,
  useSaveProductFlowSSOBatch,
  useTestProductFlowSSOConnection,
} from './productflow-sso-api'
import {
  normalizeProductFlowSSOFormValues,
  type NormalizedProductFlowSSOValues,
} from './productflow-sso-form-values'
import { ProductFlowSSOStatusCard } from './productflow-sso-status-card'
import { removeTrailingSlash } from './utils'

const createProductFlowSSOSchema = (t: (key: string) => string) =>
  z.object({
    'productflow_sso.enabled': z.string(),
    'productflow_sso.base_url': z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with http:// or https://')),
    'productflow_sso.shared_secret': z.string(),
    'productflow_sso.token_name': z.string(),
    'productflow_sso.token_group': z.string(),
    'productflow_sso.ticket_ttl_seconds': z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return false
      const parsed = Number(trimmed)
      return Number.isInteger(parsed) && parsed > 0
    }, t('Enter a positive integer')),
    'productflow_sso.session_ttl_seconds': z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return false
      const parsed = Number(trimmed)
      return Number.isInteger(parsed) && parsed > 0
    }, t('Enter a positive integer')),
    'productflow_sso.admin_session_ttl_seconds': z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return false
      const parsed = Number(trimmed)
      return Number.isInteger(parsed) && parsed > 0
    }, t('Enter a positive integer')),
  })

type ProductFlowSSOFormValues = z.output<
  ReturnType<typeof createProductFlowSSOSchema>
>
type ProductFlowSSOFormInput = z.input<
  ReturnType<typeof createProductFlowSSOSchema>
>

type ProductFlowSSOSettingsSectionProps = {
  defaultValues: {
    'productflow_sso.base_url': string
    'productflow_sso.shared_secret': string
    'productflow_sso.token_name': string
    'productflow_sso.token_group': string
    'productflow_sso.ticket_ttl_seconds': number
    'productflow_sso.session_ttl_seconds': number
    'productflow_sso.admin_session_ttl_seconds': number
    'productflow_sso.enabled': boolean
  }
}

type BaseUrlAdvisoryLevel =
  | 'ok'
  | 'warn-https'
  | 'warn-loopback'
  | 'warn-private'

type SecretStrength = 'empty' | 'weak' | 'medium' | 'strong' | 'very-strong'

const TOKEN_GROUP_CLEAR_VALUE = '__none__'
const TOKEN_GROUP_CLEAR_FALLBACK = '__productflow_sso_clear__'

function resolveTokenGroupClearValue(
  groups: string[],
  orphanCurrentValue: string | null
): string {
  const candidates = [TOKEN_GROUP_CLEAR_VALUE, TOKEN_GROUP_CLEAR_FALLBACK]
  for (const candidate of candidates) {
    if (!groups.includes(candidate) && candidate !== orphanCurrentValue) {
      return candidate
    }
  }
  let suffix = 0
  while (true) {
    const candidate = `__productflow_sso_clear_${suffix}__`
    if (!groups.includes(candidate) && candidate !== orphanCurrentValue) {
      return candidate
    }
    suffix += 1
  }
}

// scoreSecretStrength returns a coarse strength bucket the form can render
// as a 4-step meter. The thresholds intentionally favour length over
// character-class diversity because the shared secret is a server-to-server
// credential — typing characters is fine, but short secrets remain trivial
// to brute force regardless of how many symbols they contain.
function scoreSecretStrength(value: string): SecretStrength {
  const trimmed = value.trim()
  if (trimmed.length === 0) return 'empty'
  let score = 0
  if (trimmed.length >= 8) score += 1
  if (trimmed.length >= 16) score += 1
  if (trimmed.length >= 32) score += 1
  if (/[a-z]/.test(trimmed) && /[A-Z]/.test(trimmed)) score += 1
  if (/\d/.test(trimmed)) score += 1
  if (/[^a-zA-Z0-9]/.test(trimmed)) score += 1
  if (score <= 1) return 'weak'
  if (score <= 3) return 'medium'
  if (score <= 4) return 'strong'
  return 'very-strong'
}

// formatTtlSeconds returns a human-friendly approximation of a TTL value so
// admins can sanity-check the magnitude without arithmetic ("1209600" → "14
// days"). Only the dominant unit is shown; sub-units are dropped because
// the rendered string is advisory, not authoritative.
function formatTtlSeconds(raw: string): string {
  const trimmed = raw.trim()
  if (!trimmed) return ''
  const seconds = Number(trimmed)
  if (!Number.isFinite(seconds) || seconds <= 0) return ''
  if (seconds < 60) return `${seconds} s`
  if (seconds < 3600) {
    const m = Math.round((seconds / 60) * 10) / 10
    return `${m} min`
  }
  if (seconds < 86400) {
    const h = Math.round((seconds / 3600) * 10) / 10
    return `${h} h`
  }
  const d = Math.round((seconds / 86400) * 10) / 10
  return `${d} days`
}

// classifyBaseUrl flags the three "saved but operationally fragile" base URL
// shapes admins fall into during local development. Production HTTPS short-
// circuits with 'ok'; HTTP without a host (or with localhost / RFC1918 IPs)
// returns the corresponding advisory so the form can render an amber hint
// without blocking the save (PRD R11).
function classifyBaseUrl(rawUrl: string): BaseUrlAdvisoryLevel {
  const trimmed = (rawUrl ?? '').trim()
  if (!trimmed) return 'ok'
  let parsed: URL
  try {
    parsed = new URL(trimmed)
  } catch {
    return 'ok' // zod refine already surfaces invalid syntax via FormMessage.
  }
  if (parsed.protocol === 'https:') return 'ok'
  if (parsed.protocol !== 'http:') return 'ok'
  const host = parsed.hostname.toLowerCase()
  if (host === 'localhost' || host === '127.0.0.1' || host.startsWith('::1')) {
    return 'warn-loopback'
  }
  const ipv4 = host.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/)
  if (ipv4) {
    const octets = ipv4.slice(1).map(Number)
    const [a, b] = octets
    if (a === 10) return 'warn-private'
    if (a === 192 && b === 168) return 'warn-private'
    if (a === 172 && b >= 16 && b <= 31) return 'warn-private'
  }
  return 'warn-https'
}

const buildFormDefaults = (
  defaults: ProductFlowSSOSettingsSectionProps['defaultValues']
): ProductFlowSSOFormValues => ({
  'productflow_sso.enabled': String(
    defaults['productflow_sso.enabled'] ?? true
  ),
  'productflow_sso.base_url': removeTrailingSlash(
    defaults['productflow_sso.base_url'] ?? ''
  ),
  'productflow_sso.shared_secret':
    defaults['productflow_sso.shared_secret'] ?? '',
  'productflow_sso.token_name':
    defaults['productflow_sso.token_name'] ?? 'Atelier',
  'productflow_sso.token_group': defaults['productflow_sso.token_group'] ?? '',
  'productflow_sso.ticket_ttl_seconds': String(
    defaults['productflow_sso.ticket_ttl_seconds'] ?? 60
  ),
  'productflow_sso.session_ttl_seconds': String(
    defaults['productflow_sso.session_ttl_seconds'] ?? 1209600
  ),
  'productflow_sso.admin_session_ttl_seconds': String(
    defaults['productflow_sso.admin_session_ttl_seconds'] ?? 3600
  ),
})

const normalizeDefaults = (
  defaults: ProductFlowSSOSettingsSectionProps['defaultValues']
): NormalizedProductFlowSSOValues => ({
  'productflow_sso.enabled': String(
    defaults['productflow_sso.enabled'] ?? true
  ),
  'productflow_sso.base_url': removeTrailingSlash(
    defaults['productflow_sso.base_url'] ?? ''
  ),
  'productflow_sso.shared_secret':
    defaults['productflow_sso.shared_secret'] ?? '',
  'productflow_sso.token_name':
    defaults['productflow_sso.token_name'] ?? 'Atelier',
  'productflow_sso.token_group': defaults['productflow_sso.token_group'] ?? '',
  'productflow_sso.ticket_ttl_seconds': String(
    defaults['productflow_sso.ticket_ttl_seconds'] ?? 60
  ).trim(),
  'productflow_sso.session_ttl_seconds': String(
    defaults['productflow_sso.session_ttl_seconds'] ?? 1209600
  ).trim(),
  'productflow_sso.admin_session_ttl_seconds': String(
    defaults['productflow_sso.admin_session_ttl_seconds'] ?? 3600
  ).trim(),
})

export function ProductFlowSSOSettingsSection({
  defaultValues,
}: ProductFlowSSOSettingsSectionProps) {
  const { t } = useTranslation()
  const saveBatch = useSaveProductFlowSSOBatch()
  const testConnection = useTestProductFlowSSOConnection()
  const groupsQuery = useChannelGroups()
  const lastSerializedBaselineDefaults = useRef<string | null>(null)
  const [baseline, setBaseline] = useState<NormalizedProductFlowSSOValues>(() =>
    normalizeDefaults(defaultValues)
  )
  const [secretConfirmOpen, setSecretConfirmOpen] = useState(false)
  const [pendingNormalized, setPendingNormalized] =
    useState<NormalizedProductFlowSSOValues | null>(null)
  const [pendingChangedKeys, setPendingChangedKeys] = useState<
    Array<keyof NormalizedProductFlowSSOValues>
  >([])

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const schema = createProductFlowSSOSchema(t)
  const form = useForm<
    ProductFlowSSOFormInput,
    unknown,
    ProductFlowSSOFormValues
  >({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  useEffect(() => {
    const normalizedDefaults = normalizeDefaults(defaultValues)
    const serializedDefaults = JSON.stringify(normalizedDefaults)
    if (serializedDefaults === lastSerializedBaselineDefaults.current) return
    setBaseline(normalizedDefaults)
    lastSerializedBaselineDefaults.current = serializedDefaults
  }, [defaultValues])

  // beforeunload guard: warn before the operator navigates away with
  // unsaved configuration. Skipping the listener when the form is clean
  // keeps the rest of the dashboard free of phantom dialogs (PRD R10).
  useEffect(() => {
    if (!form.formState.isDirty) return
    const handler = (event: BeforeUnloadEvent) => {
      event.preventDefault()
      event.returnValue = ''
    }
    window.addEventListener('beforeunload', handler)
    return () => window.removeEventListener('beforeunload', handler)
  }, [form.formState.isDirty])

  // form.watch returns the path-narrowed value which RHF widens to `never`
  // for keys containing '.'. Cast through the schema-declared type so the
  // downstream consumers (advisory classifier, status card, test handler)
  // can treat the value as a plain string.
  const baseUrlDraft = String(form.watch('productflow_sso.base_url') ?? '')
  const baseUrlAdvisory = useMemo(
    () => classifyBaseUrl(baseUrlDraft),
    [baseUrlDraft]
  )
  const formEnabled = form.watch('productflow_sso.enabled') === 'true'

  // handleTestConnection probes ProductFlow's health endpoint using either
  // the draft base URL (preferred — lets the admin verify *before* saving)
  // or the saved configuration. Toast severity follows the 3-tier classifier
  // returned by the backend so operators can act on the result without
  // hunting through logs.
  const handleTestConnection = async () => {
    const draft = baseUrlDraft.trim()
    try {
      const result = await testConnection.mutateAsync(draft || undefined)
      const detail = `${result.message} (${result.latency_ms} ms)`
      if (result.category === 'connected') {
        toast.success(detail)
      } else if (result.category === 'network_error') {
        toast.error(detail)
      } else {
        toast.warning(detail)
      }
    } catch (error) {
      toast.error((error as Error).message || t('Test failed'))
    }
  }

  const onSubmit = async () => {
    // zodResolver returns parsed values shaped like the flat schema, while
    // react-hook-form stores dotted field names as nested raw values after an
    // edit. Diff against getValues() so saved changes are not swallowed by the
    // resolver's parsed output.
    const normalized = normalizeProductFlowSSOFormValues(form.getValues())
    // Treat an empty shared_secret as "leave existing value untouched"
    // (matches the placeholder text shown in the field). Without this
    // guard, saving any unrelated field would silently overwrite the
    // stored secret with "", breaking all subsequent SSO verifications.
    const changedKeys = (
      Object.keys(normalized) as Array<keyof NormalizedProductFlowSSOValues>
    ).filter((key) => {
      if (normalized[key] === baseline[key]) return false
      if (key === 'productflow_sso.shared_secret' && normalized[key] === '') {
        return false
      }
      return true
    })

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    // Secrets are the most consequential thing in this form: replacing one
    // invalidates every in-flight ticket and breaks every running ProductFlow
    // session that depended on the previous value. Pause and require an
    // explicit second confirmation before the batch goes through (PRD R12).
    if (changedKeys.includes('productflow_sso.shared_secret')) {
      setPendingNormalized(normalized)
      setPendingChangedKeys(changedKeys)
      setSecretConfirmOpen(true)
      return
    }

    await commitBatch(normalized, changedKeys)
  }

  const commitBatch = async (
    normalized: NormalizedProductFlowSSOValues,
    changedKeys: Array<keyof NormalizedProductFlowSSOValues>
  ) => {
    const result = await saveBatch.mutateAsync(
      changedKeys.map((key) => ({ key, value: normalized[key] }))
    )
    if (result.success) {
      const resetValues = {
        ...normalized,
        'productflow_sso.shared_secret': '',
      }
      setBaseline(resetValues)
      form.reset(resetValues)
    }
  }

  const handleConfirmSecretChange = async () => {
    setSecretConfirmOpen(false)
    if (!pendingNormalized || pendingChangedKeys.length === 0) return
    await commitBatch(pendingNormalized, pendingChangedKeys)
    setPendingNormalized(null)
    setPendingChangedKeys([])
  }

  const sharedSecretDraft = String(
    form.watch('productflow_sso.shared_secret') ?? ''
  )
  const sharedSecretStrength = useMemo(
    () => scoreSecretStrength(sharedSecretDraft),
    [sharedSecretDraft]
  )

  return (
    <SettingsSection
      title={t('ProductFlow SSO')}
      description={t(
        'Configure the New API bridge used to open ProductFlow in a new tab'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
          <ProductFlowSSOStatusCard
            formEnabled={formEnabled}
            onToggleEnabled={(next) =>
              // RHF treats keys containing '.' as nested paths, which makes
              // its setValue value-parameter widen to `never` even though the
              // schema declares a string. Match the surrounding codebase by
              // casting to bypass the nested-path narrowing (see oauth-section
              // for the same pattern).
              // eslint-disable-next-line @typescript-eslint/no-explicit-any
              form.setValue('productflow_sso.enabled' as any, String(next), {
                shouldDirty: true,
              })
            }
            baseUrlDraft={baseUrlDraft}
            isDirty={form.formState.isDirty}
          />

          {!formEnabled && (
            <Alert>
              <Info className='size-4' />
              <AlertDescription>
                {t(
                  'SSO is disabled. Saved configuration will take effect on next enable.'
                )}
              </AlertDescription>
            </Alert>
          )}

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='productflow_sso.base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('ProductFlow base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      type='url'
                      inputMode='url'
                      placeholder={t('https://image.example.com')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Public ProductFlow address used for the callback.')}
                  </FormDescription>
                  {baseUrlAdvisory !== 'ok' && (
                    <div className='flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-amber-900'>
                      <AlertTriangle className='mt-0.5 size-3.5 shrink-0' />
                      <span className='text-xs'>
                        {baseUrlAdvisory === 'warn-https' &&
                          t('Recommend HTTPS for production')}
                        {baseUrlAdvisory === 'warn-loopback' &&
                          t(
                            'Loopback detected, OK for dev, not for production'
                          )}
                        {baseUrlAdvisory === 'warn-private' &&
                          t(
                            'Private IP detected, browser may not reach across origins'
                          )}
                      </span>
                    </div>
                  )}
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='productflow_sso.token_name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Token name')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('ProductFlow')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Name of the dedicated New API token for ProductFlow.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='productflow_sso.token_group'
              render={({ field }) => {
                const groupsList = groupsQuery.data ?? []
                const orphanCurrentValue =
                  field.value && !groupsList.includes(field.value)
                    ? field.value
                    : null
                const clearValue = resolveTokenGroupClearValue(
                  groupsList,
                  orphanCurrentValue
                )

                return (
                  <FormItem>
                    <FormLabel>{t('Token group')}</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value || clearValue}
                        onValueChange={(value) =>
                          field.onChange(
                            value === clearValue ? '' : (value ?? '')
                          )
                        }
                        disabled={groupsQuery.isLoading}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue placeholder={t('Select a group')} />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            <SelectItem value={clearValue}>
                              {t('No token group')}
                            </SelectItem>
                            {orphanCurrentValue && (
                              <SelectItem
                                value={orphanCurrentValue}
                                className='text-amber-700'
                              >
                                ⚠️ {orphanCurrentValue} (
                                {t('not in current group list')})
                              </SelectItem>
                            )}
                            {groupsList.map((name) => (
                              <SelectItem key={name} value={name}>
                                {name}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    {groupsQuery.isError && (
                      <p className='text-destructive text-xs'>
                        {t('Failed to load groups, please retry.')}
                      </p>
                    )}
                    {!groupsQuery.isLoading &&
                      !groupsQuery.isError &&
                      groupsList.length === 0 && (
                        <p className='text-muted-foreground text-xs'>
                          {t(
                            'No groups available. Create one in System Settings → Models → Group Ratio.'
                          )}
                        </p>
                      )}
                    <FormDescription>
                      {t('Optional New API group assigned to the token.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )
              }}
            />

            <FormField
              control={form.control}
              name='productflow_sso.shared_secret'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SSO shared secret')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={t('Leave blank to keep the existing secret')}
                      autoComplete='new-password'
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  {sharedSecretStrength !== 'empty' && (
                    <SecretStrengthMeter level={sharedSecretStrength} t={t} />
                  )}
                  <FormDescription>
                    {t(
                      'Used by ProductFlow to verify server-to-server tickets.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='productflow_sso.ticket_ttl_seconds'
              render={({ field }) => {
                const value = String(field.value ?? '')
                const human = formatTtlSeconds(value)
                const numericValue = Number(value)
                const tooShort =
                  value.trim() !== '' &&
                  Number.isFinite(numericValue) &&
                  numericValue > 0 &&
                  numericValue < 10
                return (
                  <FormItem>
                    <FormLabel>{t('Ticket TTL (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('How long the one-time SSO ticket stays valid.')}
                      {human && (
                        <span className='text-muted-foreground ml-1'>
                          ({human})
                        </span>
                      )}
                    </FormDescription>
                    {tooShort && (
                      <div className='flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-2.5 py-1.5 text-amber-900'>
                        <AlertTriangle className='mt-0.5 size-3.5 shrink-0' />
                        <span className='text-xs'>
                          {t('Ticket TTL below 10s may break slow clients')}
                        </span>
                      </div>
                    )}
                    <FormMessage />
                  </FormItem>
                )
              }}
            />

            <FormField
              control={form.control}
              name='productflow_sso.session_ttl_seconds'
              render={({ field }) => {
                const value = String(field.value ?? '')
                const human = formatTtlSeconds(value)
                return (
                  <FormItem>
                    <FormLabel>{t('Session TTL (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Lifetime hint returned to ProductFlow after login.')}
                      {human && (
                        <span className='text-muted-foreground ml-1'>
                          ({human})
                        </span>
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )
              }}
            />

            <FormField
              control={form.control}
              name='productflow_sso.admin_session_ttl_seconds'
              render={({ field }) => {
                const value = String(field.value ?? '')
                const human = formatTtlSeconds(value)
                return (
                  <FormItem>
                    <FormLabel>{t('Admin session TTL (seconds)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        value={value}
                        onChange={(event) => field.onChange(event.target.value)}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Lifetime hint returned for admin and root ProductFlow sessions.'
                      )}
                      {human && (
                        <span className='text-muted-foreground ml-1'>
                          ({human})
                        </span>
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )
              }}
            />
          </div>

          <div className='flex flex-wrap items-center gap-2'>
            <Button type='submit' disabled={saveBatch.isPending}>
              {saveBatch.isPending
                ? t('Saving...')
                : t('Save ProductFlow settings')}
            </Button>
            <Button
              type='button'
              variant='outline'
              onClick={handleTestConnection}
              disabled={testConnection.isPending}
            >
              {testConnection.isPending
                ? t('Testing...')
                : t('Test Connection')}
            </Button>
          </div>
        </form>
      </Form>

      <AlertDialog open={secretConfirmOpen} onOpenChange={setSecretConfirmOpen}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Confirm secret change')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'This will replace the existing SSO shared secret. ProductFlow installations using the old value will stop verifying tickets until they are reconfigured. Continue?'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction onClick={handleConfirmSecretChange}>
              {t('Confirm')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}

type SecretStrengthMeterProps = {
  level: SecretStrength
  t: (key: string) => string
}

// SecretStrengthMeter renders a 4-step bar with a textual label. Hidden when
// the input is empty so the placeholder state stays uncluttered; visible
// strengths use neutral grayscale to avoid implying the meter is normative
// — it's a hint, not a gate.
function SecretStrengthMeter({ level, t }: SecretStrengthMeterProps) {
  if (level === 'empty') return null
  const filled = {
    weak: 1,
    medium: 2,
    strong: 3,
    'very-strong': 4,
  }[level]
  const label = {
    weak: t('Secret strength: weak'),
    medium: t('Secret strength: medium'),
    strong: t('Secret strength: strong'),
    'very-strong': t('Secret strength: very strong'),
  }[level]
  const color = {
    weak: 'bg-red-500',
    medium: 'bg-amber-500',
    strong: 'bg-emerald-500',
    'very-strong': 'bg-emerald-600',
  }[level]
  return (
    <div className='flex items-center gap-2'>
      <div className='flex gap-1'>
        {[0, 1, 2, 3].map((i) => (
          <span
            key={i}
            className={`h-1.5 w-6 rounded-full ${i < filled ? color : 'bg-muted'}`}
          />
        ))}
      </div>
      <span className='text-muted-foreground text-xs'>{label}</span>
    </div>
  )
}
