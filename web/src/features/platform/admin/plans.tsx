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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'

import { getHostingPlans } from '../api'
import type { HostingPlan } from '../types'
import { EditPlanDialog } from './edit-plan-dialog'

export default function PlatformPlans() {
  const { t } = useTranslation()
  const query = useQuery({
    queryKey: ['platform', 'plans'],
    queryFn: getHostingPlans,
  })
  const [selected, setSelected] = useState<HostingPlan | null>(null)
  if (query.isPending) return <LoadingState />
  if (query.isError) return <ErrorState onRetry={() => void query.refetch()} />
  return (
    <>
      <p className='text-muted-foreground'>
        {t('Plan changes apply immediately to every workspace on that plan.')}
      </p>
      <div className='grid gap-4 md:grid-cols-3'>
        {[...query.data]
          .sort((a, b) => a.limits.requests - b.limits.requests)
          .map((plan) => (
            <Card key={plan.id}>
              <CardHeader>
                <CardTitle>{plan.name}</CardTitle>
              </CardHeader>
              <CardContent className='space-y-3 text-sm'>
                <p>
                  {t('Price')}: {t(plan.price)}
                </p>
                <p>
                  {t('Monthly requests')}:{' '}
                  {plan.limits.requests.toLocaleString()}
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
                <Button variant='outline' onClick={() => setSelected(plan)}>
                  {t('Edit plan')}
                </Button>
              </CardContent>
            </Card>
          ))}
      </div>
      {selected && (
        <EditPlanDialog plan={selected} onClose={() => setSelected(null)} />
      )}
    </>
  )
}
