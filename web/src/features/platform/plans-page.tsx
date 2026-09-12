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
*/
import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { getWorkspaces } from './api'
import { HostingPlans } from './hosting-plans'
import { RedeemPlanDialog } from './redeem-plan-dialog'
import type { Workspace } from './types'

export default function PlatformPlansPage() {
  const { t } = useTranslation()
  const workspaces = useQuery({
    queryKey: ['platform', 'tenants', 'redeem'],
    queryFn: () => getWorkspaces(false, { page: 1, page_size: 100 }),
  })
  const [workspaceId, setWorkspaceId] = useState('')
  const [selected, setSelected] = useState<Workspace | null>(null)
  const tenants = workspaces.data?.tenants ?? []
  return (
    <section className='space-y-8'>
      <header className='flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between'>
        <div>
          <h1 className='text-2xl font-semibold'>{t('Plans')}</h1>
          <p className='text-muted-foreground mt-1 max-w-2xl text-sm'>
            {t(
              'Each account can create one free Lite workspace. Redeem a code to add another plan or renew an existing workspace.'
            )}
          </p>
        </div>
        <form
          className='flex w-full max-w-xl flex-col gap-3 sm:flex-row sm:items-end'
          onSubmit={(event) => {
            event.preventDefault()
            const match = tenants.find(
              (item) => String(item.tenant.id) === workspaceId
            )
            if (match) setSelected(match.tenant)
          }}
        >
          <div className='min-w-0 flex-1 space-y-2'>
            <Label htmlFor='platform-redeem-workspace'>
              {t('Apply a plan code')}
            </Label>
            <NativeSelect
              id='platform-redeem-workspace'
              className='w-full bg-background'
              value={workspaceId}
              disabled={!tenants.length}
              onChange={(event) => setWorkspaceId(event.target.value)}
            >
              <NativeSelectOption value=''>
                {tenants.length
                  ? t('Select a workspace')
                  : t('Create a workspace first')}
              </NativeSelectOption>
              {tenants.map((item) => (
                <NativeSelectOption
                  key={item.tenant.id}
                  value={String(item.tenant.id)}
                  disabled={item.tenant.status === 'suspended'}
                >
                  {item.tenant.name}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
          <Button type='submit' disabled={!workspaceId}>
            {t('Enter code')}
          </Button>
        </form>
      </header>
      <HostingPlans />
      {selected && (
        <RedeemPlanDialog
          workspace={selected}
          onClose={() => setSelected(null)}
        />
      )}
    </section>
  )
}
