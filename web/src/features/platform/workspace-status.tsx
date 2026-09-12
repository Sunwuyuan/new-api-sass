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

import { Badge } from '@/components/ui/badge'
import { Progress } from '@/components/ui/progress'

import type { Workspace } from './types'

export function WorkspaceStatus(props: { workspace: Workspace }) {
  const { t } = useTranslation()
  if (props.workspace.status === 'suspended') {
    return <Badge variant='destructive'>{t('Suspended')}</Badge>
  }
  if (
    props.workspace.plan_expires_at &&
    new Date(props.workspace.plan_expires_at).getTime() <= Date.now()
  ) {
    return <Badge variant='outline'>{t('Expired')}</Badge>
  }
  return <Badge variant='secondary'>{t('Active')}</Badge>
}

export function WorkspaceUsageMeter(props: {
  requests: number
  limit: number
  name: string
}) {
  const { t } = useTranslation()
  const percent =
    props.limit > 0
      ? Math.min(100, Math.max(0, (props.requests / props.limit) * 100))
      : 0
  return (
    <div className='min-w-32 space-y-2'>
      <p className='text-sm tabular-nums'>
        {props.requests.toLocaleString()}{' '}
        <span className='text-muted-foreground'>
          / {props.limit.toLocaleString()}
        </span>
      </p>
      <Progress
        value={percent}
        aria-label={t('Monthly usage for {{name}}', { name: props.name })}
      />
    </div>
  )
}
