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

import { getWorkspaces } from './api'
import { CreateWorkspace } from './create-workspace'
import { PlatformLink } from './navigation'

export default function PlatformCreatePage() {
  const { t } = useTranslation()
  const workspaces = useQuery({
    queryKey: ['platform', 'tenants', 'capacity'],
    queryFn: () => getWorkspaces(false, { page: 1, page_size: 1 }),
  })
  return (
    <section className='max-w-3xl space-y-6'>
      <PlatformLink
        to='/platform'
        className='text-muted-foreground text-sm hover:underline'
      >
        {t('My workspaces')}
      </PlatformLink>
      <header>
        <h1 className='text-2xl font-semibold'>{t('Create workspace')}</h1>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t('Set up an independent API workspace.')}
        </p>
      </header>
      {workspaces.isPending && <LoadingState />}
      {workspaces.isError && (
        <ErrorState onRetry={() => void workspaces.refetch()} />
      )}
      {workspaces.data && (
        <>
          <p className='text-muted-foreground text-sm'>
            {t('Workspace capacity: {{used}} / {{limit}}', {
              used: workspaces.data.workspace_count,
              limit: workspaces.data.max_workspaces,
            })}
          </p>
          <CreateWorkspace
            disabled={
              workspaces.data.workspace_count >= workspaces.data.max_workspaces
            }
          />
        </>
      )}
    </section>
  )
}
