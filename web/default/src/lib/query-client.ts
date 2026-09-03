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
import { AxiosError } from 'axios'
import {
  QueryCache,
  QueryClient,
  type QueryClientConfig,
} from '@tanstack/react-query'
import i18next from 'i18next'
import { toast } from 'sonner'
import { useAuthStore } from '@/stores/auth-store'
import { handleServerError } from '@/lib/handle-server-error'

/** Minimal router surface used by the query cache error handler. */
type RouterLike = {
  history: { location: { href: string } }
  navigate: (options: { to: string; search?: { redirect: string } }) => unknown
}

/**
 * Global TanStack Query data-fetching policy (applies to every `useQuery`):
 *
 * - `refetchOnWindowFocus: false` — switching tabs must never trigger network
 *   requests; remote data must never silently overwrite in-progress edits.
 * - `refetchOnReconnect: false`   — network recovery must not refetch either.
 * - `refetchOnMount: false`       — remounting reads from cache; no auto fetch.
 * - no `staleTime` (defaults to 0) — with every refetch trigger disabled,
 *   staleness alone can never cause a fetch.
 *
 * Remote data is therefore loaded ONLY when explicitly requested:
 *   1. cache miss (first mount of a query key),
 *   2. `queryClient.invalidateQueries(...)` after a mutation (opt-in refresh),
 *   3. `refetch()` / `queryClient.refetchQueries(...)` (manual refresh).
 *
 * Sync of remote changes is the caller's responsibility (explicit
 * invalidation); the framework must never push fresh remote values into
 * mounted components on its own.
 */
export function buildQueryClientOptions(router: RouterLike): QueryClientConfig {
  return {
    defaultOptions: {
      queries: {
        retry: (failureCount, error) => {
          // eslint-disable-next-line no-console
          if (import.meta.env.DEV) console.log({ failureCount, error })

          if (failureCount >= 0 && import.meta.env.DEV) return false
          if (failureCount > 3 && import.meta.env.PROD) return false

          return !(
            error instanceof AxiosError &&
            [401, 403].includes(error.response?.status ?? 0)
          )
        },
        // Disable ALL automatic refetch triggers (window focus / reconnect /
        // mount). Data loads only on cache miss, explicit invalidation, or
        // manual refetch — never on its own.
        refetchOnWindowFocus: false,
        refetchOnReconnect: false,
        refetchOnMount: false,
      },
      mutations: {
        onError: (error) => {
          handleServerError(error)

          if (error instanceof AxiosError) {
            if (error.response?.status === 304) {
              toast.error(i18next.t('Content not modified!'))
            }
          }
        },
      },
    },
    queryCache: new QueryCache({
      onError: (error) => {
        if (error instanceof AxiosError) {
          if (error.response?.status === 401) {
            toast.error(i18next.t('Session expired!'))
            useAuthStore.getState().auth.reset()
            const redirect = `${router.history.location.href}`
            router.navigate({ to: '/sign-in', search: { redirect } })
          }
          if (error.response?.status === 500) {
            toast.error(i18next.t('Internal Server Error!'))
            router.navigate({ to: '/500' })
          }
        }
      },
    }),
  }
}

/** Create the app-wide QueryClient with the global fetching policy applied. */
export function createQueryClient(router: RouterLike): QueryClient {
  return new QueryClient(buildQueryClientOptions(router))
}
