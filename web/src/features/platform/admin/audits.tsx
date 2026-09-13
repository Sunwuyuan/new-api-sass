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
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import { getPlatformAudits } from '../api'
import type { PlatformAudit } from '../types'
import { PlatformTable } from './table'

export default function PlatformAudits() {
  const { t } = useTranslation()
  const [selected, setSelected] = useState<PlatformAudit | null>(null)
  const columns: ColumnDef<PlatformAudit>[] = [
    { accessorKey: 'id', header: t('ID') },
    { accessorKey: 'action', header: t('Action') },
    { accessorKey: 'actor_id', header: t('Operator ID') },
    { accessorKey: 'target_id', header: t('Target ID') },
    {
      id: 'created_at',
      header: t('Time'),
      cell: ({ row }) => new Date(row.original.created_at).toLocaleString(),
    },
    {
      id: 'details',
      header: t('Details'),
      cell: ({ row }) => (
        <Button variant='ghost' onClick={() => setSelected(row.original)}>
          {t('View details')}
        </Button>
      ),
    },
  ]
  return (
    <>
      <PlatformTable
        resource='audits'
        columns={columns}
        load={async (params) => {
          const data = await getPlatformAudits(params)
          return { ...data, items: data.audits }
        }}
        searchPlaceholder={t('Search by action')}
      />
      {selected && (
        <Dialog
          open
          onOpenChange={(open) => {
            if (!open) setSelected(null)
          }}
          title={t('Platform audit')}
          description={selected.action}
        >
          <pre className='text-sm break-all whitespace-pre-wrap'>
            {selected.details}
          </pre>
        </Dialog>
      )}
    </>
  )
}
