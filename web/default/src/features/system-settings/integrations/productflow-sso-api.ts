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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { api } from '@/lib/api'

// PRODUCTFLOW_SSO_CALLBACK_PATH must stay in sync with the path baked into
// the new-api → ProductFlow handshake (see backend redirectProductFlowUser).
// The helper below mirrors Go's common.BuildURL resolution so the preview
// matches the actual redirect even when the base URL contains extra path
// segments.
export const PRODUCTFLOW_SSO_CALLBACK_PATH = '/auth/new-api/callback'

export function buildProductFlowCallbackURL(baseUrl: string): string {
  const trimmed = (baseUrl ?? '').trim()
  if (!trimmed) return ''
  try {
    return new URL(PRODUCTFLOW_SSO_CALLBACK_PATH, trimmed).toString()
  } catch {
    return ''
  }
}

// PRD R15 mandates a 4-tier classification so the dashboard can blame the
// right side: `network_error` for transport failures, `application_error`
// for ProductFlow's own refusals (HTTP non-200, supports_sso=false), and
// `other` as a catch-all for boundary cases (malformed URL, unparseable
// health body) that belong to neither side cleanly.
export type ProductFlowSSOTestCategory =
  | 'connected'
  | 'network_error'
  | 'application_error'
  | 'other'

export type ProductFlowSSOTestResult = {
  ok: boolean
  category: ProductFlowSSOTestCategory
  latency_ms: number
  tested_against: 'draft' | 'saved'
  tested_at: number
  message: string
}

export type ProductFlowSSOStatus = {
  enabled: boolean
  configured: boolean
  redis_enabled: boolean
  callback_url_preview: string
  last_test_result: ProductFlowSSOTestResult | null
}

type StatusResponse = {
  success: boolean
  data: ProductFlowSSOStatus
}

type TestResponse = {
  success: boolean
  data: ProductFlowSSOTestResult
}

type BatchUpdate = {
  key: string
  value: string
}

type BatchResponse = {
  success: boolean
  message?: string
  failed_keys?: string[]
}

export async function fetchProductFlowSSOStatus(): Promise<ProductFlowSSOStatus> {
  const res = await api.get<StatusResponse>('/api/productflow/sso/status')
  return res.data.data
}

export async function testProductFlowSSOConnection(
  baseUrl?: string
): Promise<ProductFlowSSOTestResult> {
  const body = baseUrl ? { base_url: baseUrl } : {}
  const res = await api.post<TestResponse>('/api/productflow/sso/test', body)
  return res.data.data
}

export async function saveProductFlowSSOBatch(
  updates: BatchUpdate[]
): Promise<BatchResponse> {
  const res = await api.put<BatchResponse>('/api/option/batch', { updates })
  return res.data
}

export function useProductFlowSSOStatus() {
  return useQuery({
    queryKey: ['productflow-sso-status'],
    queryFn: fetchProductFlowSSOStatus,
    staleTime: 30_000,
    refetchOnWindowFocus: false,
  })
}

export function useTestProductFlowSSOConnection() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (baseUrl?: string) => testProductFlowSSOConnection(baseUrl),
    onSuccess: () => {
      // Refresh the status snapshot so the "last test" badge reflects the
      // outcome without forcing the admin to manually reload the page.
      queryClient.invalidateQueries({ queryKey: ['productflow-sso-status'] })
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Test failed'))
    },
  })
}

export function useSaveProductFlowSSOBatch() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (updates: BatchUpdate[]) => saveProductFlowSSOBatch(updates),
    onSuccess: (data) => {
      if (data.success) {
        queryClient.invalidateQueries({ queryKey: ['system-options'] })
        queryClient.invalidateQueries({ queryKey: ['productflow-sso-status'] })
        toast.success(i18next.t('Setting updated successfully'))
      } else {
        toast.error(data.message || i18next.t('Failed to update setting'))
      }
    },
    onError: (error: Error) => {
      toast.error(error.message || i18next.t('Failed to update setting'))
    },
  })
}
