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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { LoadingState } from '@/components/loading-state'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'

import {
  createWildcardDomain,
  deleteWildcardDomain,
  getAdminWildcardDomains,
  updateWildcardDomain,
} from '../api'
import type { WildcardDomain } from '../types'

export function AdminWildcardDomains() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const query = useQuery({
    queryKey: ['platform', 'admin', 'wildcard-domains'],
    queryFn: getAdminWildcardDomains,
  })
  const [domain, setDomain] = useState('')
  const [pendingDelete, setPendingDelete] = useState<WildcardDomain | null>(
    null
  )
  const createDomain = useMutation({
    meta: { errorToast: false },
    mutationFn: () =>
      createWildcardDomain({ domain: domain.trim(), enabled: true }),
    onSuccess: async () => {
      setDomain('')
      toast.success(t('Wildcard domain added'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const toggleDomain = useMutation({
    mutationFn: (item: WildcardDomain) =>
      updateWildcardDomain(item.id, { enabled: !item.enabled }),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const removeDomain = useMutation({
    meta: { errorToast: false },
    mutationFn: (id: number) => deleteWildcardDomain(id),
    onSuccess: async () => {
      setPendingDelete(null)
      toast.success(t('Wildcard domain removed'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Wildcard domains')}</CardTitle>
        <CardDescription>
          {t(
            'Users create prefixes under these domains, such as shop.example.com.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        {query.isPending && <LoadingState />}
        {query.isError && (
          <p role='alert' className='text-destructive text-sm'>
            {query.error.message}
          </p>
        )}
        <ul className='space-y-3'>
          {(query.data ?? []).map((item) => (
            <li
              key={item.id}
              className='flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3'
            >
              <div className='min-w-0'>
                <p className='font-medium break-all'>{item.domain}</p>
                <Field orientation='horizontal' className='mt-2 w-auto'>
                  <Checkbox
                    id={`wildcard-enabled-${item.id}`}
                    checked={item.enabled}
                    disabled={toggleDomain.isPending}
                    onCheckedChange={() => toggleDomain.mutate(item)}
                  />
                  <FieldLabel htmlFor={`wildcard-enabled-${item.id}`}>
                    {t('Enabled')}
                  </FieldLabel>
                </Field>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => {
                  removeDomain.reset()
                  setPendingDelete(item)
                }}
              >
                {t('Remove')}
              </Button>
            </li>
          ))}
        </ul>
        <form
          className='space-y-3'
          onSubmit={(event) => {
            event.preventDefault()
            if (!domain.trim()) return
            createDomain.mutate()
          }}
        >
          <Field>
            <FieldLabel htmlFor='wildcard-domain-input'>
              {t('Wildcard domain')}
            </FieldLabel>
            <Input
              id='wildcard-domain-input'
              value={domain}
              onChange={(event) => setDomain(event.target.value)}
              placeholder='example.com'
              autoComplete='off'
              disabled={createDomain.isPending}
            />
            <FieldDescription>
              {t(
                'Enter the suffix only. Users then bind prefixes such as docs.example.com.'
              )}
            </FieldDescription>
            {createDomain.isError && (
              <FieldError>{createDomain.error.message}</FieldError>
            )}
          </Field>
          <Button
            type='submit'
            disabled={createDomain.isPending || !domain.trim()}
          >
            {t('Add wildcard domain')}
          </Button>
        </form>
      </CardContent>
      {pendingDelete && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !removeDomain.isPending) setPendingDelete(null)
          }}
          title={t('Remove wildcard domain')}
          desc={pendingDelete.domain}
          destructive
          confirmText={t('Remove')}
          handleConfirm={() => removeDomain.mutate(pendingDelete.id)}
          isLoading={removeDomain.isPending}
        >
          {removeDomain.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {removeDomain.error.message}
            </p>
          )}
        </ConfirmDialog>
      )}
    </Card>
  )
}
