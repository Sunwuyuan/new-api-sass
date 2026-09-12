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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { platformStatusQuery } from './auth-api'
import type { PlatformRouter } from './router'

export function PlatformBrand() {
  const { t } = useTranslation()
  const status = useQuery(platformStatusQuery)
  return (
    <Link<PlatformRouter>
      to='/platform'
      className='flex min-w-0 items-center gap-2 transition-opacity hover:opacity-80'
    >
      <img
        src={status.data?.logo || '/logo.png'}
        alt={t('Logo')}
        className='size-8 shrink-0 rounded-full object-cover'
      />
      <span className='truncate text-lg font-medium'>
        {status.data?.system_name || 'New API SaaS'}
      </span>
    </Link>
  )
}
