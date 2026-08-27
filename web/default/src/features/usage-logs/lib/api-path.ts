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
/**
 * API path helpers for the usage logs feature.
 *
 * The backend registers log / midjourney / task endpoints as a Gin route group
 * with explicit sub-paths, e.g. `/api/task/` for admins (AdminAuth) and
 * `/api/task/self` for the current user (UserAuth). Because `self` is a path
 * segment and not a suffix, the base endpoint must keep its trailing slash —
 * `/api/task` + `self` produces `/api/taskself`, which matches no route and
 * makes the request fail with 404.
 *
 * These helpers normalise the trailing slash so callers cannot reintroduce
 * that bug.
 */

/**
 * Ensure an endpoint path ends with exactly one trailing slash.
 */
export function withTrailingSlash(endpoint: string): string {
  return endpoint.endsWith('/') ? endpoint : `${endpoint}/`
}

/**
 * Build the list endpoint for a route group.
 *
 * @param endpoint Group path, e.g. `/api/task/` or `/api/task`
 * @param isAdmin  When false the `self` sub-path is appended
 */
export function buildApiPath(endpoint: string, isAdmin: boolean): string {
  const base = withTrailingSlash(endpoint)
  return isAdmin ? base : `${base}self`
}

/**
 * Build the stats endpoint for a route group.
 *
 * Admins read aggregate stats across all users (`/api/log/stat`), regular
 * users only their own (`/api/log/self/stat`).
 */
export function buildStatsPath(endpoint: string, isAdmin: boolean): string {
  const base = withTrailingSlash(endpoint)
  return isAdmin ? `${base}stat` : `${base}self/stat`
}
