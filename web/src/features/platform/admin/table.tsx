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
import {
  getCoreRowModel,
  useReactTable,
  type ColumnDef,
  type PaginationState,
} from '@tanstack/react-table'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage } from '@/components/data-table'
import { ErrorState } from '@/components/error-state'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import { platformSessionQuery } from '../api'
import type { PageParams, PageResult } from '../types'

export function PlatformTable<T>(props: {
  resource: string
  scope?: 'user' | 'admin'
  columns: ColumnDef<T>[]
  load: (params: PageParams) => Promise<PageResult & { items: T[] }>
  statuses?: { value: string; label: string }[]
  roles?: { value: string; label: string }[]
  searchable?: boolean
  actions?: ReactNode
  searchPlaceholder?: string
}) {
  const { t } = useTranslation()
  const session = useQuery(platformSessionQuery)
  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: 20,
  })
  const [search, setSearch] = useState('')
  const [status, setStatus] = useState('')
  const [role, setRole] = useState('')
  const params = {
    page: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    search,
    status,
    role,
  }
  const query = useQuery({
    queryKey: [
      'platform',
      props.scope ?? 'admin',
      props.resource,
      session.data?.user.id,
      params,
    ],
    queryFn: () => props.load(params),
    enabled:
      !!session.data &&
      !session.data.user.must_change_password &&
      (props.scope === 'user' ||
        ['admin', 'root'].includes(session.data.user.role)),
  })
  const table = useReactTable({
    data: query.data?.items ?? [],
    columns: props.columns,
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    manualFiltering: true,
    rowCount: query.data?.pagination.total ?? 0,
    state: { pagination, globalFilter: search },
    onPaginationChange: setPagination,
    onGlobalFilterChange: (value) => {
      setSearch(String(value))
      setPagination((current) => ({ ...current, pageIndex: 0 }))
    },
  })
  return (
    <div className='space-y-4'>
      {query.isError && (
        <ErrorState
          title={t('Unable to complete the request')}
          description={query.error.message}
          onRetry={() => void query.refetch()}
        />
      )}
      <DataTablePage
        table={table}
        columns={props.columns}
        isLoading={query.isPending}
        isFetching={query.isFetching}
        fixedHeight={false}
        paginationInFooter={false}
        emptyTitle={t('No results')}
        toolbarProps={{
          hideViewOptions: true,
          customSearch: props.searchable === false ? null : undefined,
          searchPlaceholder: props.searchPlaceholder ?? t('Search'),
          searchDebounceMs: 300,
          preActions: props.actions,
          hasAdditionalFilters: status !== '' || role !== '',
          onReset: () => {
            setStatus('')
            setRole('')
            setPagination((current) => ({ ...current, pageIndex: 0 }))
          },
          additionalSearch: (
            <>
              {props.statuses && (
                <NativeSelect
                  aria-label={t('Status')}
                  value={status}
                  onChange={(event) => {
                    setStatus(event.target.value)
                    setPagination((current) => ({ ...current, pageIndex: 0 }))
                  }}
                >
                  <NativeSelectOption value=''>
                    {t('All statuses')}
                  </NativeSelectOption>
                  {props.statuses.map((option) => (
                    <NativeSelectOption key={option.value} value={option.value}>
                      {option.label}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              )}
              {props.roles && (
                <NativeSelect
                  aria-label={t('Role')}
                  value={role}
                  onChange={(event) => {
                    setRole(event.target.value)
                    setPagination((current) => ({ ...current, pageIndex: 0 }))
                  }}
                >
                  <NativeSelectOption value=''>
                    {t('All roles')}
                  </NativeSelectOption>
                  {props.roles.map((option) => (
                    <NativeSelectOption key={option.value} value={option.value}>
                      {option.label}
                    </NativeSelectOption>
                  ))}
                </NativeSelect>
              )}
            </>
          ),
        }}
      />
    </div>
  )
}
