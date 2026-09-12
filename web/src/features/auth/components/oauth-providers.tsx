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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  IconDiscord,
  IconGithub,
  IconLinuxDo,
  IconTelegram,
  IconWeChat,
} from '@/assets/brand-icons'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { useOAuthLogin } from '../hooks/use-oauth-login'
import type { SystemStatus } from '../types'

export type OAuthProviderStatus = Pick<
  SystemStatus,
  | 'github_oauth'
  | 'discord_oauth'
  | 'oidc_enabled'
  | 'oidc_display_name'
  | 'linuxdo_oauth'
  | 'telegram_oauth'
  | 'wechat_login'
> & { custom_oauth_providers?: Array<{ slug: string; name: string }> }

type ProviderButtonsProps = {
  status: OAuthProviderStatus | null
  disabled?: boolean
  className?: string
  onWeChatLogin?: () => void
  isWeChatLoading?: boolean
  onProviderLogin: (provider: string) => void
  isLoading?: boolean
  githubButtonText?: string
  githubButtonDisabled?: boolean
}

type OAuthProvidersProps = Omit<
  ProviderButtonsProps,
  | 'status'
  | 'onProviderLogin'
  | 'isLoading'
  | 'githubButtonText'
  | 'githubButtonDisabled'
> & { status: SystemStatus | null; redirectTo?: string }

export function OAuthProviders(props: OAuthProvidersProps) {
  const oauth = useOAuthLogin(props.status, props.redirectTo)
  const handlers: Record<string, () => void> = {
    github: oauth.handleGitHubLogin,
    discord: oauth.handleDiscordLogin,
    oidc: oauth.handleOIDCLogin,
    linuxdo: oauth.handleLinuxDOLogin,
    telegram: oauth.handleTelegramLogin,
  }
  return (
    <OAuthProviderButtons
      {...props}
      isLoading={oauth.isLoading}
      githubButtonText={oauth.githubButtonText}
      githubButtonDisabled={oauth.githubButtonDisabled}
      onProviderLogin={(slug) => {
        if (handlers[slug]) {
          handlers[slug]()
          return
        }
        const custom = props.status?.custom_oauth_providers?.find(
          (p) => p.slug === slug
        )
        if (custom) void oauth.handleCustomOAuthLogin(custom)
      }}
    />
  )
}

type ProviderButton = {
  key: string
  label: string
  onClick: () => void
  icon?: ReactNode
  disabled?: boolean
}

// The button presentation is shared; the caller owns its account scope.
export function OAuthProviderButtons(props: ProviderButtonsProps) {
  const { t } = useTranslation()
  const status = props.status
  const providerButtons: ProviderButton[] = []

  if (status?.wechat_login && props.onWeChatLogin) {
    providerButtons.push({
      key: 'wechat',
      label: t('Continue with WeChat'),
      onClick: props.onWeChatLogin,
      icon: <IconWeChat className='h-4 w-4' aria-hidden='true' />,
      disabled: props.isWeChatLoading,
    })
  }

  if (status?.github_oauth) {
    providerButtons.push({
      key: 'github',
      label: props.githubButtonText || t('Continue with GitHub'),
      onClick: () => props.onProviderLogin('github'),
      icon: <IconGithub className='h-4 w-4' aria-hidden='true' />,
      disabled: props.githubButtonDisabled,
    })
  }

  if (status?.discord_oauth) {
    providerButtons.push({
      key: 'discord',
      label: t('Continue with Discord'),
      onClick: () => props.onProviderLogin('discord'),
      icon: <IconDiscord className='h-4 w-4' aria-hidden='true' />,
    })
  }

  if (status?.oidc_enabled) {
    const oidcDisplayName = status.oidc_display_name?.trim() || 'OIDC'
    providerButtons.push({
      key: 'oidc',
      label: t('Continue with {{name}}', {
        name: oidcDisplayName,
      }),
      onClick: () => props.onProviderLogin('oidc'),
    })
  }

  if (status?.linuxdo_oauth) {
    providerButtons.push({
      key: 'linuxdo',
      label: t('Continue with LinuxDO'),
      onClick: () => props.onProviderLogin('linuxdo'),
      icon: <IconLinuxDo className='h-4 w-4' aria-hidden='true' />,
    })
  }

  if (status?.telegram_oauth) {
    providerButtons.push({
      key: 'telegram',
      label: t('Continue with Telegram'),
      onClick: () => props.onProviderLogin('telegram'),
      icon: <IconTelegram data-icon='inline-start' aria-hidden='true' />,
    })
  }

  // Custom OAuth providers
  const customProviders = status?.custom_oauth_providers
  if (customProviders && customProviders.length > 0) {
    for (const provider of customProviders) {
      providerButtons.push({
        key: `custom-${provider.slug}`,
        label: t('Continue with {{name}}', { name: provider.name }),
        onClick: () => props.onProviderLogin(provider.slug),
      })
    }
  }

  if (providerButtons.length === 0) return null

  return (
    <div className={cn('space-y-3', props.className)}>
      <div className='relative'>
        <div className='absolute inset-0 flex items-center'>
          <span className='w-full border-t' />
        </div>
        <div className='relative flex justify-center text-xs uppercase'>
          <span className='bg-background text-muted-foreground px-2'>
            {t('Or continue with')}
          </span>
        </div>
      </div>

      <div className='flex flex-col gap-2'>
        {providerButtons.map(
          ({ key, label, onClick, icon, disabled: extraDisabled }) => (
            <Button
              key={key}
              variant='outline'
              type='button'
              disabled={props.disabled || props.isLoading || extraDisabled}
              onClick={onClick}
              className='h-11 w-full justify-center gap-2 rounded-lg'
            >
              {icon}
              {label}
            </Button>
          )
        )}
      </div>
    </div>
  )
}
