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
import { platformClient } from './api'
import type { HostingPlan, PlanAssignment, Workspace } from './types'

export type UsageMonth = { month: string; requests: number }
export type WorkspaceDetail = {
  tenant: Workspace
  owner_name: string
  plan: HostingPlan
  usage: UsageMonth
  history: UsageMonth[]
  assignments: Array<PlanAssignment & { created_at: string }>
}
export type UsageSummary = {
  month: string
  summary: {
    workspaces: number
    active: number
    suspended: number
    expired: number
    expiring_soon: number
    requests: number
    exhausted: number
  }
  plans: Array<{
    plan_id: number
    name: string
    workspaces: number
    requests: number
  }>
  history: UsageMonth[]
}
export async function getWorkspaceDetail(
  id: number,
  admin: boolean
): Promise<WorkspaceDetail> {
  return (
    await platformClient.get<WorkspaceDetail>(
      `${admin ? '/admin' : ''}/tenants/${id}`
    )
  ).data
}
export function platformUsageQuery(admin: boolean) {
  return {
    queryKey: ['platform', 'usage', admin],
    queryFn: async (): Promise<UsageSummary> =>
      (await platformClient.get<UsageSummary>(`${admin ? '/admin' : ''}/usage`))
        .data,
  }
}
