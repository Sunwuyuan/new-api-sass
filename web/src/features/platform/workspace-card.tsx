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
import { useTranslation } from 'react-i18next'

import { Button, buttonVariants } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import { WorkspaceLink } from './navigation'
import type { HostingPlan, WorkspaceUsage } from './types'
import { WorkspaceUsageMeter } from './workspace-status'

export function WorkspaceCard(props: {
  item: WorkspaceUsage
  plans: HostingPlan[]
}) {
  const { t } = useTranslation()
  const workspace = props.item.tenant
  const currentPlan = props.plans.find((plan) => plan.id === workspace.plan_id)
  const expired =
    workspace.plan_expires_at !== null &&
    new Date(workspace.plan_expires_at).getTime() <= Date.now()
  let statusText = t('Active')
  if (expired) statusText = t('Expired')
  if (workspace.status === 'suspended') statusText = t('Suspended')
  return (
    <Card>
      <CardHeader>
        <CardTitle className='break-words'>{workspace.name}</CardTitle>
        <CardDescription className='break-words'>
          /t/{workspace.slug} · {currentPlan?.name} · {statusText}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <WorkspaceUsageMeter
          requests={props.item.usage.requests}
          limit={currentPlan?.limits.requests ?? 0}
          name={workspace.name}
        />
        <p>
          {t('Plan expires')}:{' '}
          {workspace.plan_expires_at
            ? new Date(workspace.plan_expires_at).toLocaleString()
            : t('No expiry')}
        </p>
        <div className='flex flex-wrap gap-2'>
          <Button
            render={<a href={`/t/${workspace.slug}/`} />}
            nativeButton={false}
            role='link'
            disabled={expired || workspace.status === 'suspended'}
          >
            {t('Enter workspace')}
          </Button>
          <WorkspaceLink
            id={workspace.id}
            className={buttonVariants({ variant: 'outline' })}
          >
            {t('Manage workspace')}
          </WorkspaceLink>
        </div>
      </CardContent>
    </Card>
  )
}
