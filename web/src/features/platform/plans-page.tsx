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
import { Ticket02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { getWorkspaces } from './api'
import { HostingPlans } from './hosting-plans'
import { RedeemPlanDialog } from './redeem-plan-dialog'

export default function PlatformPlansPage() {
  const { t } = useTranslation()
  const workspaces = useQuery({
    queryKey: ['platform', 'tenants', 'redeem'],
    queryFn: () => getWorkspaces(false, { page: 1, page_size: 100 }),
  })
  const [redeemOpen, setRedeemOpen] = useState(false)
  const workspaceList = (workspaces.data?.tenants ?? []).map(
    (item) => item.tenant
  )
  const redeemable = workspaceList.some(
    (workspace) => workspace.status !== 'suspended'
  )
  return (
    <section className='space-y-8'>
      <header className='flex flex-wrap items-end justify-between gap-4'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('Plans')}</h1>
          <p className='text-muted-foreground mt-1 max-w-2xl text-sm'>
            {t(
              'Each account can create one free Lite workspace. Redeem a code to add another plan or renew an existing workspace.'
            )}
          </p>
        </div>
        <Button
          onClick={() => setRedeemOpen(true)}
          disabled={!redeemable}
          title={redeemable ? undefined : t('Create a workspace first')}
        >
          <HugeiconsIcon icon={Ticket02Icon} data-icon='inline-start' />
          {t('Redeem a code')}
        </Button>
      </header>
      <HostingPlans
        workspaces={workspaceList}
        onRedeem={() => setRedeemOpen(true)}
      />
      {redeemOpen && (
        <RedeemPlanDialog
          workspaces={workspaceList}
          onClose={() => setRedeemOpen(false)}
        />
      )}
    </section>
  )
}
