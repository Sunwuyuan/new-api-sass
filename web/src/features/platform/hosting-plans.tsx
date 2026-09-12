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
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { getHostingPlans } from './api'

export function HostingPlans() {
  const { t } = useTranslation()
  const plans = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  if (plans.isPending) return <LoadingState />
  if (plans.isError) return <ErrorState onRetry={() => void plans.refetch()} />
  return (
    <section
      aria-label={t('Hosting plans')}
      className='grid gap-4 md:grid-cols-3'
    >
      {[...plans.data]
        .sort((a, b) => a.limits.requests - b.limits.requests)
        .map((plan) => (
          <Card key={plan.id}>
            <CardHeader>
              <CardTitle>
                {plan.name} · {t(plan.price)}
              </CardTitle>
            </CardHeader>
            <CardContent className='space-y-2 text-sm'>
              <p>
                {t('Monthly requests')}: {plan.limits.requests.toLocaleString()}
              </p>
              <p>
                {t('Users')}: {plan.limits.users.toLocaleString()} ·{' '}
                {t('Tokens')}: {plan.limits.tokens.toLocaleString()} ·{' '}
                {t('Channels')}: {plan.limits.channels.toLocaleString()}
              </p>
              <p>
                {t('Workspaces per account')}:{' '}
                {plan.capabilities.max_workspaces}
              </p>
              <p>
                {plan.capabilities.custom_branding
                  ? t('Custom branding')
                  : t('Basic branding')}
              </p>
              <p>
                {plan.capabilities.remove_platform_footer
                  ? t('Custom platform footer')
                  : t('Platform footer required')}
              </p>
            </CardContent>
          </Card>
        ))}
    </section>
  )
}
