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
import type { TFunction } from 'i18next'
import { z } from 'zod'

export function platformPasswordSchema(t: TFunction) {
  return z.string().refine((password) => {
    const length = [...password].length
    return length >= 15 && length <= 128
  }, t('Use 15 to 128 characters for a new password.'))
}

export function platformCountSchema(t: TFunction, maximum: number) {
  const message = t('Enter a whole number from {{min}} to {{max}}.', {
    min: 1,
    max: maximum,
  })
  return z
    .number({ error: message })
    .int(message)
    .min(1, message)
    .max(maximum, message)
}
