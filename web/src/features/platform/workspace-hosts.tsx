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
import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'

import {
  createWorkspaceHost,
  deleteWorkspaceHost,
  getWildcardDomains,
  verifyWorkspaceHost,
} from './api'
import type { WildcardDomain, WorkspaceHost } from './types'

const prefixPattern = /^[a-z0-9](?:[a-z0-9-]{0,46}[a-z0-9])?$/

export function WorkspaceHosts(props: {
  workspaceId: number
  admin: boolean
  hosts: WorkspaceHost[]
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const wildcards = useQuery({
    queryKey: ['platform', 'wildcard-domains'],
    queryFn: getWildcardDomains,
  })
  const [prefix, setPrefix] = useState('')
  const [domainId, setDomainId] = useState('')
  const [customHost, setCustomHost] = useState('')
  const [method, setMethod] = useState<'txt' | 'cname'>('txt')
  const [pendingDelete, setPendingDelete] = useState<WorkspaceHost | null>(null)
  const domains = wildcards.data ?? []
  const selectedDomain =
    domains.find((domain) => String(domain.id) === domainId) ?? domains[0]
  const createHost = useMutation({
    meta: { errorToast: false },
    mutationFn: (input: Parameters<typeof createWorkspaceHost>[1]) =>
      createWorkspaceHost(props.workspaceId, input, props.admin),
    onSuccess: async () => {
      setPrefix('')
      setCustomHost('')
      toast.success(t('Domain added'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const verifyHost = useMutation({
    meta: { errorToast: false },
    mutationFn: (hostId: number) =>
      verifyWorkspaceHost(props.workspaceId, hostId, props.admin),
    onSuccess: async () => {
      toast.success(t('Domain verified'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  const removeHost = useMutation({
    meta: { errorToast: false },
    mutationFn: (hostId: number) =>
      deleteWorkspaceHost(props.workspaceId, hostId, props.admin),
    onSuccess: async () => {
      setPendingDelete(null)
      toast.success(t('Domain removed'))
      await queryClient.invalidateQueries({ queryKey: ['platform'] })
    },
  })
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Domains')}</CardTitle>
      </CardHeader>
      <CardContent className='space-y-6'>
        <p className='text-muted-foreground text-sm'>
          {t(
            'Prefixes are independent of the workspace ID. Custom domains can point at the server or a reverse proxy you run yourself.'
          )}
        </p>
        <ul className='space-y-4'>
          {props.hosts.length === 0 && (
            <li className='text-muted-foreground text-sm'>
              {t('No domains yet')}
            </li>
          )}
          {props.hosts.map((host) => (
            <HostRow
              key={host.id}
              host={host}
              verifying={
                verifyHost.isPending && verifyHost.variables === host.id
              }
              onVerify={() => verifyHost.mutate(host.id)}
              onDelete={() => {
                removeHost.reset()
                setPendingDelete(host)
              }}
              verifyError={
                verifyHost.variables === host.id
                  ? verifyHost.error?.message
                  : undefined
              }
            />
          ))}
        </ul>
        {domains.length > 0 && (
          <form
            className='space-y-3'
            onSubmit={(event) => {
              event.preventDefault()
              if (!selectedDomain || !prefixPattern.test(prefix)) return
              createHost.mutate({
                kind: 'wildcard',
                prefix,
                wildcard_domain_id: selectedDomain.id,
              })
            }}
          >
            <Field data-invalid={!prefixPattern.test(prefix) && prefix !== ''}>
              <FieldLabel htmlFor='workspace-host-prefix'>
                {t('Workspace prefix')}
              </FieldLabel>
              <div className='flex flex-wrap items-center gap-2'>
                <Input
                  id='workspace-host-prefix'
                  value={prefix}
                  onChange={(event) => setPrefix(event.target.value)}
                  disabled={createHost.isPending}
                  placeholder='docs'
                  className='min-w-0 flex-1'
                />
                <span className='text-muted-foreground'>.</span>
                <DomainSelect
                  id='workspace-host-wildcard'
                  domains={domains}
                  value={selectedDomain?.id}
                  onChange={setDomainId}
                  disabled={createHost.isPending}
                />
              </div>
              <FieldDescription>
                {t(
                  'Use lowercase letters, numbers and hyphens, up to 48 characters.'
                )}
              </FieldDescription>
            </Field>
            <Button
              type='submit'
              disabled={createHost.isPending || !prefixPattern.test(prefix)}
            >
              {t('Add prefix')}
            </Button>
          </form>
        )}
        <form
          className='space-y-3'
          onSubmit={(event) => {
            event.preventDefault()
            if (!customHost.trim()) return
            createHost.mutate({
              kind: 'custom',
              host: customHost.trim(),
              method,
            })
          }}
        >
          <Field>
            <FieldLabel htmlFor='workspace-custom-host'>
              {t('Custom domain')}
            </FieldLabel>
            <Input
              id='workspace-custom-host'
              value={customHost}
              onChange={(event) => setCustomHost(event.target.value)}
              disabled={createHost.isPending}
              placeholder='api.example.com'
              autoComplete='off'
            />
            <FieldDescription>
              {t(
                'Point this hostname to the server or reverse-proxy it yourself, then add a TXT or CNAME record to prove you own it.'
              )}
            </FieldDescription>
          </Field>
          <Field>
            <FieldLabel htmlFor='workspace-verify-method'>
              {t('Verification method')}
            </FieldLabel>
            <NativeSelect
              id='workspace-verify-method'
              value={method}
              onChange={(event) =>
                setMethod(event.target.value === 'cname' ? 'cname' : 'txt')
              }
              disabled={createHost.isPending}
            >
              <NativeSelectOption value='txt'>TXT</NativeSelectOption>
              <NativeSelectOption value='cname'>CNAME</NativeSelectOption>
            </NativeSelect>
          </Field>
          {createHost.isError && (
            <FieldError>{createHost.error.message}</FieldError>
          )}
          <Button
            type='submit'
            variant='outline'
            disabled={createHost.isPending || !customHost.trim()}
          >
            {t('Add custom domain')}
          </Button>
        </form>
      </CardContent>
      {pendingDelete && (
        <ConfirmDialog
          open
          onOpenChange={(open) => {
            if (!open && !removeHost.isPending) setPendingDelete(null)
          }}
          title={t('Remove domain')}
          desc={pendingDelete.host}
          destructive
          confirmText={t('Remove domain')}
          handleConfirm={() => removeHost.mutate(pendingDelete.id)}
          isLoading={removeHost.isPending}
        >
          {removeHost.isError && (
            <p role='alert' className='text-destructive text-sm'>
              {removeHost.error.message}
            </p>
          )}
        </ConfirmDialog>
      )}
    </Card>
  )
}

function DomainSelect(props: {
  id: string
  domains: WildcardDomain[]
  value?: number
  onChange: (value: string) => void
  disabled?: boolean
}) {
  const { t } = useTranslation()
  if (props.domains.length === 1) {
    return (
      <span id={props.id} className='text-muted-foreground'>
        {props.domains[0].domain}
      </span>
    )
  }
  return (
    <NativeSelect
      id={props.id}
      value={props.value ? String(props.value) : ''}
      onChange={(event) => props.onChange(event.target.value)}
      disabled={props.disabled}
      aria-label={t('Wildcard domain')}
    >
      {props.domains.map((domain) => (
        <NativeSelectOption key={domain.id} value={String(domain.id)}>
          {domain.domain}
        </NativeSelectOption>
      ))}
    </NativeSelect>
  )
}

function HostRow(props: {
  host: WorkspaceHost
  verifying: boolean
  onVerify: () => void
  onDelete: () => void
  verifyError?: string
}) {
  const { t } = useTranslation()
  const host = props.host
  const pending = host.kind === 'custom' && host.status !== 'verified'
  return (
    <li className='space-y-3 rounded-lg border p-3'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='min-w-0 space-y-1'>
          <p className='font-medium break-all'>
            {host.url ? (
              <a href={host.url} className='hover:underline'>
                {host.host}
              </a>
            ) : (
              host.host
            )}
          </p>
          <div className='flex flex-wrap gap-2'>
            <Badge variant='secondary'>
              {host.kind === 'wildcard' ? t('Prefix') : t('Custom domain')}
            </Badge>
            <Badge
              variant={host.status === 'verified' ? 'secondary' : 'outline'}
            >
              {host.status === 'verified' ? t('Verified') : t('Pending')}
            </Badge>
          </div>
        </div>
        <div className='flex flex-wrap gap-2'>
          {pending && (
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.verifying}
              onClick={props.onVerify}
            >
              {t('Check DNS')}
            </Button>
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={props.onDelete}
          >
            {t('Remove')}
          </Button>
        </div>
      </div>
      {pending && (
        <FieldGroup className='text-sm'>
          {host.verification_method !== 'cname' &&
            host.txt_name &&
            host.txt_value && (
              <DnsRecord
                label={t('TXT record')}
                name={host.txt_name}
                value={host.txt_value}
              />
            )}
          {host.verification_method !== 'txt' &&
            host.cname_host &&
            host.cname_target && (
              <DnsRecord
                label={t('CNAME record')}
                name={host.cname_host}
                value={host.cname_target}
              />
            )}
          {props.verifyError && (
            <p role='alert' className='text-destructive'>
              {props.verifyError}
            </p>
          )}
        </FieldGroup>
      )}
    </li>
  )
}

function DnsRecord(props: { label: string; name: string; value: string }) {
  const { t } = useTranslation()
  return (
    <div className='space-y-1'>
      <p className='font-medium'>{props.label}</p>
      <p className='text-muted-foreground break-all'>
        {t('Name')}: {props.name}
      </p>
      <p className='flex items-start gap-2 break-all'>
        <span className='min-w-0 flex-1'>
          {t('Value')}: {props.value}
        </span>
        <CopyButton value={props.value} tooltip={t('Copy to clipboard')} />
      </p>
    </div>
  )
}
