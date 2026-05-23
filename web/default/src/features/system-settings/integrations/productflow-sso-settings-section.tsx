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
import { useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Textarea } from '@/components/ui/textarea'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { removeTrailingSlash } from './utils'

const createProductFlowSSOSchema = (t: (key: string) => string) =>
  z.object({
    'productflow_sso.base_url': z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with http:// or https://')),
    'productflow_sso.shared_secret': z.string(),
    'productflow_sso.token_name': z.string(),
    'productflow_sso.token_model_limits': z.string(),
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
  })

type ProductFlowSSOFormValues = z.infer<
  ReturnType<typeof createProductFlowSSOSchema>
>

type ProductFlowSSOSettingsSectionProps = {
  defaultValues: {
    'productflow_sso.base_url': string
    'productflow_sso.shared_secret': string
    'productflow_sso.token_name': string
    'productflow_sso.token_model_limits': string
    'productflow_sso.token_group': string
    'productflow_sso.ticket_ttl_seconds': number
    'productflow_sso.session_ttl_seconds': number
  }
}

type NormalizedProductFlowSSOValues = {
  'productflow_sso.base_url': string
  'productflow_sso.shared_secret': string
  'productflow_sso.token_name': string
  'productflow_sso.token_model_limits': string
  'productflow_sso.token_group': string
  'productflow_sso.ticket_ttl_seconds': string
  'productflow_sso.session_ttl_seconds': string
}

const normalizeCsv = (value: string) =>
  value
    .split(',')
    .map((item) => item.trim())
    .filter(Boolean)
    .join(',')

const buildFormDefaults = (
  defaults: ProductFlowSSOSettingsSectionProps['defaultValues']
): ProductFlowSSOFormValues => ({
  'productflow_sso.base_url': removeTrailingSlash(
    defaults['productflow_sso.base_url'] ?? ''
  ),
  'productflow_sso.shared_secret':
    defaults['productflow_sso.shared_secret'] ?? '',
  'productflow_sso.token_name':
    defaults['productflow_sso.token_name'] ?? 'ProductFlow',
  'productflow_sso.token_model_limits':
    defaults['productflow_sso.token_model_limits'] ?? '',
  'productflow_sso.token_group':
    defaults['productflow_sso.token_group'] ?? '',
  'productflow_sso.ticket_ttl_seconds': String(
    defaults['productflow_sso.ticket_ttl_seconds'] ?? 60
  ),
  'productflow_sso.session_ttl_seconds': String(
    defaults['productflow_sso.session_ttl_seconds'] ?? 1209600
  ),
})

const normalizeDefaults = (
  defaults: ProductFlowSSOSettingsSectionProps['defaultValues']
): NormalizedProductFlowSSOValues => ({
  'productflow_sso.base_url': removeTrailingSlash(
    defaults['productflow_sso.base_url'] ?? ''
  ),
  'productflow_sso.shared_secret':
    defaults['productflow_sso.shared_secret'] ?? '',
  'productflow_sso.token_name':
    defaults['productflow_sso.token_name'] ?? 'ProductFlow',
  'productflow_sso.token_model_limits': normalizeCsv(
    defaults['productflow_sso.token_model_limits'] ?? ''
  ),
  'productflow_sso.token_group':
    defaults['productflow_sso.token_group'] ?? '',
  'productflow_sso.ticket_ttl_seconds': String(
    defaults['productflow_sso.ticket_ttl_seconds'] ?? 60
  ).trim(),
  'productflow_sso.session_ttl_seconds': String(
    defaults['productflow_sso.session_ttl_seconds'] ?? 1209600
  ).trim(),
})

const normalizeFormValues = (
  values: ProductFlowSSOFormValues
): NormalizedProductFlowSSOValues => ({
  'productflow_sso.base_url': removeTrailingSlash(
    values['productflow_sso.base_url']
  ),
  'productflow_sso.shared_secret':
    values['productflow_sso.shared_secret'].trim(),
  'productflow_sso.token_name': values['productflow_sso.token_name'].trim(),
  'productflow_sso.token_model_limits': normalizeCsv(
    values['productflow_sso.token_model_limits']
  ),
  'productflow_sso.token_group':
    values['productflow_sso.token_group'].trim(),
  'productflow_sso.ticket_ttl_seconds':
    values['productflow_sso.ticket_ttl_seconds'].trim(),
  'productflow_sso.session_ttl_seconds':
    values['productflow_sso.session_ttl_seconds'].trim(),
})

export function ProductFlowSSOSettingsSection({
  defaultValues,
}: ProductFlowSSOSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<NormalizedProductFlowSSOValues>(
    normalizeDefaults(defaultValues)
  )

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )

  const schema = createProductFlowSSOSchema(t)
  const form = useForm<ProductFlowSSOFormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  useResetForm(form, formDefaults)

  const onSubmit = async (values: ProductFlowSSOFormValues) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof NormalizedProductFlowSSOValues>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
  }

  return (
    <SettingsSection
      title={t('ProductFlow SSO')}
      description={t(
        'Configure the New API bridge used to open ProductFlow in a new tab'
      )}
    >
      <Form {...form}>
        <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-6'>
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
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Token group')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('image')}
                      {...field}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Optional New API group assigned to the token.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
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
                  <FormDescription>
                    {t('Used by ProductFlow to verify server-to-server tickets.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='productflow_sso.token_model_limits'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Token model whitelist')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={4}
                    placeholder={t('gpt-image-1, veo-3, seedance-1')}
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Comma-separated models allowed for the ProductFlow token.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='productflow_sso.ticket_ttl_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Ticket TTL (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('How long the one-time SSO ticket stays valid.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='productflow_sso.session_ttl_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Session TTL (seconds)')}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={1}
                      step={1}
                      value={field.value}
                      onChange={(event) => field.onChange(event.target.value)}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Lifetime hint returned to ProductFlow after login.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <Button type='submit' disabled={updateOption.isPending}>
            {updateOption.isPending
              ? t('Saving...')
              : t('Save ProductFlow settings')}
          </Button>
        </form>
      </Form>
    </SettingsSection>
  )
}
