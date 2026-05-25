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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'
import { normalizeProductFlowSSOFormValues } from './productflow-sso-form-values.ts'

const flatValues = {
  'productflow_sso.enabled': 'true',
  'productflow_sso.base_url': 'https://image.example.test/',
  'productflow_sso.shared_secret': '',
  'productflow_sso.token_name': 'Atelier',
  'productflow_sso.token_group': '',
  'productflow_sso.image_model': '',
  'productflow_sso.ticket_ttl_seconds': '60',
  'productflow_sso.session_ttl_seconds': '1209600',
  'productflow_sso.admin_session_ttl_seconds': '3600',
}

describe('normalizeProductFlowSSOFormValues', () => {
  test('normalizes flat option keys from the saved defaults', () => {
    const normalized = normalizeProductFlowSSOFormValues(flatValues)

    assert.equal(
      normalized['productflow_sso.base_url'],
      'https://image.example.test'
    )
    assert.equal(normalized['productflow_sso.token_group'], '')
    assert.equal(normalized['productflow_sso.image_model'], '')
  })

  test('prefers React Hook Form nested dotted-path values over stale flat values', () => {
    const normalized = normalizeProductFlowSSOFormValues({
      ...flatValues,
      productflow_sso: {
        token_group: 'GPT-PLUS',
        image_model: ' gpt-image-2 ',
        ticket_ttl_seconds: ' 120 ',
      },
    })

    assert.equal(normalized['productflow_sso.token_group'], 'GPT-PLUS')
    assert.equal(normalized['productflow_sso.image_model'], 'gpt-image-2')
    assert.equal(normalized['productflow_sso.ticket_ttl_seconds'], '120')
  })

  test('keeps an empty nested token group as an intentional clear', () => {
    const normalized = normalizeProductFlowSSOFormValues({
      ...flatValues,
      'productflow_sso.token_group': 'old-group',
      productflow_sso: {
        token_group: '',
      },
    })

    assert.equal(normalized['productflow_sso.token_group'], '')
  })
})
