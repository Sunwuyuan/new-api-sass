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
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'

import { WorkspaceLink } from './navigation'
import type { HostingPlan, WorkspaceUsage } from './types'
import { WorkspaceStatus, WorkspaceUsageMeter } from './workspace-status'

export function WorkspaceCard(props: {
  item: WorkspaceUsage
  plans: HostingPlan[]
}) {
  const { t } = useTranslation()
  const workspace = props.item.tenant
  const expired =
    workspace.plan_expires_at !== null &&
    new Date(workspace.plan_expires_at).getTime() <= Date.now()
  const currentPlan = expired
    ? props.plans.find((plan) => plan.name === 'Lite')
    : props.plans.find((plan) => plan.id === workspace.plan_id)
  return (
    <Card>
      <CardHeader>
        <div className='flex items-start justify-between gap-3'>
          <CardTitle className='break-words'>{workspace.name}</CardTitle>
          <WorkspaceStatus workspace={workspace} />
        </div>
        <CardDescription className='break-words'>
          /t/{workspace.slug}
          {currentPlan ? ` · ${currentPlan.name}` : ''}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div>
          <p className='text-muted-foreground mb-2 text-xs'>
            {t('Monthly requests')}
          </p>
          <WorkspaceUsageMeter
            requests={props.item.usage.requests}
            limit={currentPlan?.limits.requests ?? 0}
            name={workspace.name}
          />
        </div>
        <p className='text-sm'>
          {t('Plan expires')}:{' '}
          {workspace.plan_expires_at
            ? new Date(workspace.plan_expires_at).toLocaleString()
            : t('No expiry')}
        </p>
        {expired && (
          <p className='text-muted-foreground text-sm'>
            {t(
              'This workspace now uses Lite. Existing users can keep working. Renew the plan to create tokens, channels and more users.'
            )}
          </p>
        )}
      </CardContent>
      <CardFooter className='gap-2'>
        <Button
          render={<a href={`/t/${workspace.slug}/`} />}
          nativeButton={false}
          role='link'
          disabled={workspace.status === 'suspended'}
        >
          {t('Enter workspace')}
        </Button>
        <WorkspaceLink
          id={workspace.id}
          className={buttonVariants({ variant: 'outline' })}
        >
          {t('Manage workspace')}
        </WorkspaceLink>
      </CardFooter>
    </Card>
  )
}
