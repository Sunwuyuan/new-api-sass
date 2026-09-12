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
import { Key01Icon, Loading03Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useIsMutating, useMutation } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { OAuthProviderButtons } from '@/features/auth/components/oauth-providers'
import { WeChatLoginDialog } from '@/features/auth/components/wechat-login-dialog'
import { requestPasskeyAssertion } from '@/features/auth/passkey/assertion'
import { isPasskeySupported } from '@/lib/passkey'
import { AuthOperationError } from '@/lib/secure-verification'

import {
  beginPlatformOAuth,
  beginPlatformPasskey,
  finishPlatformPasskey,
  platformWeChatLogin,
  type PlatformAuthStatus,
  type PlatformAuthIntent,
} from './auth-api'
import type { PlatformSession } from './types'

export function PlatformLoginMethods(props: {
  status: PlatformAuthStatus
  intent?: PlatformAuthIntent
  redirectTo: string
  disabled?: boolean
  showPasskey?: boolean
  onSuccess: (session: PlatformSession) => Promise<void> | void
}) {
  const { t } = useTranslation()
  const [wechatOpen, setWeChatOpen] = useState(false)
  const [code, setCode] = useState('')
  const [supported, setSupported] = useState(false)
  const operation = useRef<AbortController | null>(null)
  const busy = useIsMutating({ mutationKey: ['platform', 'authenticate'] }) > 0
  const intent = props.intent ?? 'login'
  useEffect(() => {
    let active = true
    void isPasskeySupported()
      .then((value) => {
        if (active) setSupported(value)
      })
      .catch(() => {
        if (active) setSupported(false)
      })
    return () => {
      active = false
      operation.current?.abort()
    }
  }, [])
  const oauth = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: async (provider: string) => {
      const url = await beginPlatformOAuth(provider, intent, props.redirectTo)
      window.location.assign(url)
    },
  })
  const wechat = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: () =>
      platformWeChatLogin(code.trim(), intent, props.redirectTo),
    onSuccess: async (session) => {
      setCode('')
      setWeChatOpen(false)
      await props.onSuccess(session)
    },
  })
  const passkey = useMutation({
    mutationKey: ['platform', 'authenticate'],
    meta: { errorToast: false },
    mutationFn: async () => {
      if (operation.current || intent === 'link') {
        throw new Error(t('A security operation is already in progress.'))
      }
      const controller = new AbortController()
      operation.current = controller
      try {
        const assertion = await requestPasskeyAssertion(
          () => beginPlatformPasskey(intent, controller.signal),
          controller.signal
        )
        return await finishPlatformPasskey(
          intent,
          assertion.flowToken,
          assertion.assertion,
          controller.signal
        )
      } catch (error) {
        throw AuthOperationError.from(error, t('Passkey login failed'))
      } finally {
        operation.current = null
      }
    },
    onSuccess: props.onSuccess,
  })
  return (
    <div className='space-y-4'>
      {props.showPasskey !== false &&
        props.status.passkey_login &&
        intent !== 'link' && (
          <div className='space-y-1'>
            <Button
              type='button'
              variant='outline'
              disabled={props.disabled || busy || !supported}
              onClick={() => passkey.mutate()}
              className='h-11 w-full justify-center gap-2 rounded-lg'
            >
              {passkey.isPending ? (
                <HugeiconsIcon
                  icon={Loading03Icon}
                  className='animate-spin'
                  data-icon='inline-start'
                />
              ) : (
                <HugeiconsIcon icon={Key01Icon} data-icon='inline-start' />
              )}
              {t('Sign in with Passkey')}
            </Button>
            {!supported && (
              <p className='text-muted-foreground text-xs'>
                {t('Passkey is not supported on this device.')}
              </p>
            )}
          </div>
        )}
      <OAuthProviderButtons
        status={props.status}
        disabled={props.disabled || busy}
        onProviderLogin={(provider) => oauth.mutate(provider)}
        onWeChatLogin={() => {
          wechat.reset()
          setWeChatOpen(true)
        }}
      />
      {oauth.isError && (
        <p role='alert' className='text-destructive text-sm'>
          {oauth.error.message}
        </p>
      )}
      {passkey.isError && (
        <p role='alert' className='text-destructive text-sm'>
          {passkey.error.message}
        </p>
      )}
      <WeChatLoginDialog
        open={wechatOpen}
        onOpenChange={(open) => {
          if (!busy) {
            setWeChatOpen(open)
            setCode('')
          }
        }}
        code={code}
        onCodeChange={setCode}
        qrCodeUrl={props.status.wechat_qrcode}
        onSubmit={() => wechat.mutate()}
        isSubmitting={wechat.isPending}
        disabled={props.disabled}
        error={wechat.error?.message}
      />
    </div>
  )
}
