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
import { removeTrailingSlash } from './utils.ts'

export type NormalizedProductFlowSSOValues = {
  'productflow_sso.enabled': string
  'productflow_sso.base_url': string
  'productflow_sso.shared_secret': string
  'productflow_sso.token_name': string
  'productflow_sso.token_group': string
  'productflow_sso.image_model': string
  'productflow_sso.ticket_ttl_seconds': string
  'productflow_sso.session_ttl_seconds': string
  'productflow_sso.admin_session_ttl_seconds': string
}

function readDottedStringValue(values: object, key: string): string {
  const record = values as Record<string, unknown>
  const nestedValue = key.split('.').reduce<unknown>((current, segment) => {
    if (!current || typeof current !== 'object') return undefined
    const currentRecord = current as Record<string, unknown>
    return Object.prototype.hasOwnProperty.call(currentRecord, segment)
      ? currentRecord[segment]
      : undefined
  }, record)
  const value = nestedValue !== undefined ? nestedValue : record[key]

  if (typeof value === 'string') return value
  if (value == null) return ''
  return String(value)
}

export function normalizeProductFlowSSOFormValues(
  values: object
): NormalizedProductFlowSSOValues {
  const read = (key: keyof NormalizedProductFlowSSOValues) =>
    readDottedStringValue(values, key)

  return {
    'productflow_sso.enabled': read('productflow_sso.enabled'),
    'productflow_sso.base_url': removeTrailingSlash(
      read('productflow_sso.base_url')
    ),
    'productflow_sso.shared_secret': read(
      'productflow_sso.shared_secret'
    ).trim(),
    'productflow_sso.token_name': read('productflow_sso.token_name').trim(),
    'productflow_sso.token_group': read('productflow_sso.token_group').trim(),
    'productflow_sso.image_model': read('productflow_sso.image_model').trim(),
    'productflow_sso.ticket_ttl_seconds': read(
      'productflow_sso.ticket_ttl_seconds'
    ).trim(),
    'productflow_sso.session_ttl_seconds': read(
      'productflow_sso.session_ttl_seconds'
    ).trim(),
    'productflow_sso.admin_session_ttl_seconds': read(
      'productflow_sso.admin_session_ttl_seconds'
    ).trim(),
  }
}
