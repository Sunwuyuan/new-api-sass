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
import {
  focusManager,
  onlineManager,
  queryOptions,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query'
import type { TFunction } from 'i18next'
import { useEffect, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'

import { useStatus } from '@/hooks/use-status'
import { handleServerError } from '@/lib/handle-server-error'
import { ROLE } from '@/lib/roles'
import { statusQueryOptions } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'

import {
  fetchLatestSystemRelease,
  UpdateCheckError,
  type UpdateCheckErrorCode,
} from './api'
import { compareSystemVersions, parseSystemVersion } from './releases'
import {
  subscribeSystemUpdatePreferences,
  useSystemUpdatePreferencesStore,
  useSystemUpdateStore,
  type SystemUpdateSnapshot,
} from './store'

function hostedUpdateCheckDisabled(
  status: { hosted?: unknown; update_check_disabled?: unknown } | null | undefined
): boolean {
  return status?.update_check_disabled === true || status?.hosted === true
}

function hostedRelease(version: string | undefined) {
  if (!version || !parseSystemVersion(version)) return null
  return {
    tag_name: version,
    prerelease: false,
    body: null,
    published_at: null,
  }
}
export const SYSTEM_UPDATE_INTERVAL = 60 * 60 * 1000
export const SYSTEM_UPDATE_QUERY_KEY = ['system-update'] as const

export function getUpdateErrorMessage(
  code: UpdateCheckErrorCode,
  t: TFunction
): string {
  switch (code) {
    case 'rate-limit':
      return t('GitHub rate limit reached. Try again later.')
    case 'timeout':
      return t('Update check timed out. Try again.')
    case 'payload':
      return t('Unexpected release payload')
    default:
      return t('Failed to check for updates')
  }
}

export const systemUpdateQueryOptions = queryOptions({
  queryKey: SYSTEM_UPDATE_QUERY_KEY,
  queryFn: async ({ signal, client }): Promise<SystemUpdateSnapshot> => {
    const cached = client.getQueryData(statusQueryOptions.queryKey) as
      | { hosted?: unknown; update_check_disabled?: unknown; version?: unknown }
      | undefined
    const now = Date.now()
    const statusState = client.getQueryState(statusQueryOptions.queryKey)
    const statusAge = now - (statusState?.dataUpdatedAt ?? 0)
    const statusFresh =
      statusState?.data != null &&
      statusAge >= 0 &&
      statusAge < 5 * 60 * 1000
    let status = cached
    if (!hostedUpdateCheckDisabled(cached) && !statusFresh) {
      // Refresh the running server version too, so an open tab notices upgrades.
      status =
        (await client
          .fetchQuery({ ...statusQueryOptions, meta: { errorToast: false } })
          .catch(() => cached)) ?? cached
    }
    if (hostedUpdateCheckDisabled(status)) {
      const version =
        typeof status?.version === 'string' ? status.version.trim() : undefined
      const snapshot: SystemUpdateSnapshot = {
        release: hostedRelease(version),
        lastCheckedAt: now,
        lastAttemptAt: now,
        error: null,
      }
      useSystemUpdateStore.getState().setSnapshot(snapshot)
      return snapshot
    }
    let snapshot: SystemUpdateSnapshot
    try {
      const release = await fetchLatestSystemRelease(signal)
      signal.throwIfAborted()
      snapshot = {
        release,
        lastCheckedAt: now,
        lastAttemptAt: now,
        error: null,
      }
    } catch (error) {
      if (signal.aborted) throw error
      const previous = useSystemUpdateStore.getState().snapshot
      // A failed attempt is also cached, so focus changes and remounts cannot
      // hammer GitHub while it is unavailable or rate limiting this browser.
      snapshot = {
        release: previous?.release ?? null,
        lastCheckedAt: previous?.lastCheckedAt ?? 0,
        lastAttemptAt: Date.now(),
        error: error instanceof UpdateCheckError ? error.code : 'network',
      }
    }
    useSystemUpdateStore.getState().setSnapshot(snapshot)
    return snapshot
  },
  initialData: () => useSystemUpdateStore.getState().snapshot ?? undefined,
  initialDataUpdatedAt: () =>
    useSystemUpdateStore.getState().snapshot?.lastAttemptAt,
  staleTime: SYSTEM_UPDATE_INTERVAL,
  gcTime: 24 * SYSTEM_UPDATE_INTERVAL,
  retry: false,
  meta: { errorToast: false },
  refetchOnWindowFocus: true,
  refetchOnReconnect: true,
  refetchIntervalInBackground: false,
  refetchInterval: (query) => {
    if (query.state.fetchStatus === 'fetching') return false
    return Math.max(
      1_000,
      SYSTEM_UPDATE_INTERVAL -
        (Date.now() - (query.state.data?.lastAttemptAt ?? 0))
    )
  },
})

function isPageVisible(): boolean {
  return focusManager.isFocused()
}

function isBrowserOnline(): boolean {
  return (
    onlineManager.isOnline() &&
    (typeof navigator === 'undefined' || navigator.onLine)
  )
}

export function useSystemUpdate() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const isAdmin = (user?.role ?? 0) >= ROLE.ADMIN
  const visible = useSyncExternalStore(focusManager.subscribe, isPageVisible)
  const online = useSyncExternalStore(onlineManager.subscribe, isBrowserOnline)
  const { status } = useStatus()
  const hosted = hostedUpdateCheckDisabled(status)
  const queryClient = useQueryClient()
  const query = useQuery({
    ...systemUpdateQueryOptions,
    enabled: isAdmin && visible && online && !hosted,
  })

  useEffect(() => {
    if (!isAdmin) {
      void queryClient.cancelQueries({ queryKey: SYSTEM_UPDATE_QUERY_KEY })
    }
  }, [isAdmin, queryClient])

  const version = status?.version?.trim()
  const currentVersion =
    version === 'v0.0.0' || version === '0.0.0'
      ? undefined
      : version || undefined
  const release = query.data?.release ?? null
  const comparison = compareSystemVersions(currentVersion, release?.tag_name)
  const hasUpdate = !hosted && comparison === -1
  const isIgnored = useSyncExternalStore(subscribeSystemUpdatePreferences, () =>
    Boolean(
      user &&
      release &&
      useSystemUpdatePreferencesStore
        .getState()
        .ignoredVersionsByUserId[user.id]?.includes(release.tag_name)
    )
  )

  const setIgnored = (ignored: boolean) => {
    if (!isAdmin || !user || !release) return
    useSystemUpdatePreferencesStore
      .getState()
      .setVersionIgnored(user.id, release.tag_name, ignored)
  }

  const checkNow = async () => {
    if (!isAdmin || !online || hosted) return
    const result = await query.refetch({ cancelRefetch: false })
    if (result.data?.error) {
      handleServerError(new Error(getUpdateErrorMessage(result.data.error, t)))
    }
  }

  return {
    currentVersion,
    release,
    comparison,
    hasUpdate,
    isIgnored,
    shouldNotify: isAdmin && hasUpdate && !isIgnored,
    setIgnored,
    checking: query.isFetching,
    online,
    snapshot: query.data,
    checkNow,
    hosted,
  }
}
