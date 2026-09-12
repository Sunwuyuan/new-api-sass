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
import { Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useId } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Field, FieldLabel } from '@/components/ui/field'
import { Input } from '@/components/ui/input'

export function WeChatLoginDialog(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  code: string
  onCodeChange: (value: string) => void
  qrCodeUrl?: string
  isSubmitting: boolean
  disabled?: boolean
  error?: string
  onSubmit: () => void
}) {
  const { t } = useTranslation()
  const id = useId()
  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('WeChat sign in')}
      description={t(
        'Scan the QR code to follow the official account and reply with “验证码” to receive your verification code.'
      )}
      contentClassName='max-w-sm'
      headerClassName='text-left'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.isSubmitting}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form={id}
            disabled={
              props.isSubmitting || !props.code.trim() || props.disabled
            }
          >
            {props.isSubmitting && (
              <HugeiconsIcon
                icon={Loading03Icon}
                className='animate-spin'
                data-icon='inline-start'
              />
            )}
            {t('Confirm')}
          </Button>
        </>
      }
    >
      {props.qrCodeUrl ? (
        <div className='flex justify-center'>
          <img
            src={props.qrCodeUrl}
            alt={t('WeChat login QR code')}
            className='h-40 w-40 rounded-md border object-contain'
          />
        </div>
      ) : (
        <p className='text-muted-foreground text-sm'>
          {t('QR code is not configured. Please contact support.')}
        </p>
      )}
      <form
        id={id}
        onSubmit={(event) => {
          event.preventDefault()
          if (!props.isSubmitting && !props.disabled && props.code.trim()) {
            props.onSubmit()
          }
        }}
      >
        {props.error && (
          <p role='alert' className='text-destructive mb-3 text-sm'>
            {props.error}
          </p>
        )}
        <Field>
          <FieldLabel htmlFor={`${id}-code`}>
            {t('Verification code')}
          </FieldLabel>
          <Input
            id={`${id}-code`}
            placeholder={t('Enter the verification code')}
            value={props.code}
            onChange={(event) => props.onCodeChange(event.target.value)}
            autoComplete='one-time-code'
            disabled={props.isSubmitting}
          />
        </Field>
      </form>
    </Dialog>
  )
}
