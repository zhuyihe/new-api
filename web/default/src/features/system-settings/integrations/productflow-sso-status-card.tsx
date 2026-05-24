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
import { useState } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Check, Copy, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import {
  buildProductFlowCallbackURL,
  useProductFlowSSOStatus,
  type ProductFlowSSOTestResult,
} from './productflow-sso-api'

type StatusKind =
  | 'connected'
  | 'disabled'
  | 'not_configured'
  | 'configuration_error'

type ProductFlowSSOStatusCardProps = {
  formEnabled: boolean
  onToggleEnabled: (next: boolean) => void
  baseUrlDraft: string
  isDirty: boolean
}

// resolveStatusKind collapses the (enabled, configured) tuple into the four
// states the operator actually needs to see at a glance. The "configuration
// error" branch fires when the admin saved partial config (missing secret
// despite a base URL, etc.) — i.e. the server-side row exists but
// validateForStart would refuse a redirect.
function resolveStatusKind(
  formEnabled: boolean,
  serverEnabled: boolean | undefined,
  configured: boolean | undefined,
  baseUrlDraft: string
): StatusKind {
  if (!formEnabled) return 'disabled'
  if (!configured) {
    return baseUrlDraft.trim() === '' ? 'not_configured' : 'configuration_error'
  }
  if (serverEnabled === false) return 'disabled'
  return 'connected'
}

function statusBadgeProps(kind: StatusKind, t: (key: string) => string) {
  switch (kind) {
    case 'connected':
      return {
        label: t('Connected'),
        className: 'bg-emerald-100 text-emerald-900 border-emerald-200',
      }
    case 'disabled':
      return {
        label: t('Disabled'),
        className: 'bg-muted text-muted-foreground border-border',
      }
    case 'not_configured':
      return {
        label: t('Not configured'),
        className: 'bg-amber-100 text-amber-900 border-amber-200',
      }
    case 'configuration_error':
      return {
        label: t('Configuration incomplete'),
        className: 'bg-amber-100 text-amber-900 border-amber-200',
      }
  }
}

function lastTestResultBadgeProps(
  result: ProductFlowSSOTestResult,
  t: (key: string) => string
) {
  if (result.ok) {
    return {
      label: t('Connected'),
      className: 'bg-emerald-100 text-emerald-900 border-emerald-200',
    }
  }
  switch (result.category) {
    case 'network_error':
      return {
        label: t('Error'),
        className: 'bg-rose-100 text-rose-900 border-rose-200',
      }
    case 'application_error':
      return {
        label: t('Warning'),
        className: 'bg-amber-100 text-amber-900 border-amber-200',
      }
    default:
      return {
        label: t('Other'),
        className: 'bg-muted text-muted-foreground border-border',
      }
  }
}

// isClientValidBaseURL mirrors the backend's isProductFlowBaseURLValid check
// so the dashboard never tells the admin "connected" while the saved value
// would refuse a real redirect. We accept only absolute http(s) URLs.
function isClientValidBaseURL(raw: string): boolean {
  const trimmed = (raw ?? '').trim()
  if (!trimmed) return false
  try {
    const parsed = new URL(trimmed)
    if (!parsed.host) return false
    return parsed.protocol === 'http:' || parsed.protocol === 'https:'
  } catch {
    return false
  }
}

export function ProductFlowSSOStatusCard({
  formEnabled,
  onToggleEnabled,
  baseUrlDraft,
  isDirty,
}: ProductFlowSSOStatusCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const { data: status, isFetching } = useProductFlowSSOStatus()
  const [copied, setCopied] = useState(false)

  // Draft-first preview: as the admin edits the base URL, the preview must
  // reflect their *current* input, not the server's last-saved snapshot.
  // The server preview is only used as a fallback when the field is empty
  // (e.g. immediately after page load before the form populates). When the
  // draft is non-empty but not a valid URL we suppress the preview entirely
  // rather than render a misleading concatenation like "typo/auth/...".
  const draftTrimmed = baseUrlDraft.trim()
  const draftEmpty = draftTrimmed === ''
  const draftURLValid = isClientValidBaseURL(baseUrlDraft)
  const draftPreview =
    !draftEmpty && draftURLValid
      ? buildProductFlowCallbackURL(baseUrlDraft)
      : ''
  const callbackPreview =
    draftPreview || (draftEmpty ? (status?.callback_url_preview ?? '') : '')
  const lastTestResult = status?.last_test_result
  const lastTestResultBadge = lastTestResult
    ? lastTestResultBadgeProps(lastTestResult, t)
    : null

  // Effective `configured` overrides the server value with the draft's URL
  // validity. Without this, editing a saved base_url into an invalid string
  // would leave the badge stuck on "connected" until save (bug report).
  // Secret presence still comes from the server because the form never sees
  // the stored secret in plaintext.
  const effectiveConfigured = draftEmpty
    ? !!status?.configured
    : draftURLValid && !!status?.configured
  const statusKind = resolveStatusKind(
    formEnabled,
    status?.enabled,
    effectiveConfigured,
    baseUrlDraft
  )
  const badge = statusBadgeProps(statusKind, t)
  const redisWarning =
    effectiveConfigured && status?.redis_enabled === false && formEnabled

  const handleCopy = async () => {
    if (!callbackPreview) return
    try {
      await navigator.clipboard.writeText(callbackPreview)
      setCopied(true)
      window.setTimeout(() => setCopied(false), 1500)
    } catch {
      toast.error(t('Copy failed'))
    }
  }

  const handleRefresh = () => {
    queryClient.invalidateQueries({ queryKey: ['productflow-sso-status'] })
  }

  return (
    <div className='bg-card space-y-3 rounded-lg border p-4'>
      <div className='flex flex-wrap items-center gap-3'>
        <Switch
          checked={formEnabled}
          onCheckedChange={onToggleEnabled}
          aria-label={t('Enable ProductFlow SSO')}
        />
        <div className='text-foreground text-base font-medium'>
          {t('ProductFlow SSO')}
        </div>
        <Badge variant='outline' className={cn('border', badge.className)}>
          {badge.label}
        </Badge>
        {isDirty && (
          <Badge
            variant='outline'
            className='border-amber-200 bg-amber-50 text-amber-900'
          >
            {t('Unsaved changes')}
          </Badge>
        )}
        <div className='ml-auto'>
          <TooltipProvider delay={200}>
            <Tooltip>
              <TooltipTrigger
                render={
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    onClick={handleRefresh}
                    aria-label={t('Refresh status')}
                    disabled={isFetching}
                  >
                    <RefreshCw
                      className={cn('size-4', isFetching && 'animate-spin')}
                    />
                  </Button>
                }
              />
              <TooltipContent>{t('Refresh status')}</TooltipContent>
            </Tooltip>
          </TooltipProvider>
        </div>
      </div>

      {callbackPreview && (
        <div className='bg-muted/40 flex items-center gap-2 rounded-md px-3 py-2'>
          <span className='text-muted-foreground text-xs font-medium uppercase'>
            {t('Callback URL')}
          </span>
          <code className='text-foreground flex-1 truncate text-sm'>
            {callbackPreview}
          </code>
          <Button
            type='button'
            variant='ghost'
            size='icon'
            onClick={handleCopy}
            aria-label={t('Copy')}
          >
            {copied ? (
              <Check className='size-4 text-emerald-600' />
            ) : (
              <Copy className='size-4' />
            )}
          </Button>
        </div>
      )}

      {lastTestResult && lastTestResultBadge && (
        <div className='bg-muted/40 space-y-2 rounded-md border px-3 py-2'>
          <div className='flex flex-wrap items-center gap-2'>
            <span className='text-muted-foreground text-xs font-medium uppercase'>
              {t('Status')}
            </span>
            <Badge
              variant='outline'
              className={cn('border', lastTestResultBadge.className)}
            >
              {lastTestResultBadge.label}
            </Badge>
            <span className='text-muted-foreground text-xs font-medium uppercase'>
              {t('Source')}
            </span>
            <Badge
              variant='outline'
              className='border-border bg-background text-muted-foreground capitalize'
            >
              {lastTestResult.tested_against}
            </Badge>
            <span className='text-muted-foreground ml-auto text-xs font-medium uppercase'>
              {t('Time')}
            </span>
            <span className='text-foreground text-xs'>
              {formatTimestampToDate(lastTestResult.tested_at)}
            </span>
          </div>
          <div className='text-muted-foreground text-xs font-medium uppercase'>
            {t('Details')}
          </div>
          <div className='text-foreground text-sm break-words'>
            {lastTestResult.message}
            <span className='text-muted-foreground ml-1'>
              ({lastTestResult.latency_ms} ms)
            </span>
          </div>
        </div>
      )}

      {redisWarning && (
        <div className='flex items-start gap-2 rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-amber-900'>
          <AlertTriangle className='mt-0.5 size-4 shrink-0' />
          <div className='text-sm'>
            {t('Redis not enabled (single-process fallback mode)')}
          </div>
        </div>
      )}
    </div>
  )
}
