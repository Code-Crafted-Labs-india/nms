export type MetricPoint = { time: string; device: string; value: number }
export type Device = { id:number; hostname:string; ipAddress:string; type:string; location:string; status:'online'|'offline'; rttMs:number|null; lastUpdate:string|null }
export type NetworkEvent = { time:string; device:string; severity:'INFO'|'WARNING'|'CRITICAL'; type:string; message:string }
export type TopologyEdge = { id:string; source:string; target:string; sourcePort:string; targetPort:string }
export type DashboardData = { generatedAt:string; overview:{ monitored:number; online:number; offline:number; openAlerts:number }; devices:Device[]; events:NetworkEvent[]; latency:MetricPoint[]; cpu:MetricPoint[]; memory:MetricPoint[]; topology:TopologyEdge[] }
export class ApiError extends Error {
  status: number
  constructor(status:number) { super(`API request failed: ${status}`); this.status = status }
}
let csrfToken = ''
async function request<T>(path:string, init:RequestInit={}) {
  const response = await fetch(path, { credentials:'same-origin', cache:'no-store', ...init, headers:{ ...(init.body?{'Content-Type':'application/json'}:{}), ...(csrfToken?{'X-CSRF-Token':csrfToken}:{}), ...init.headers } })
  if(!response.ok) throw new ApiError(response.status)
  if(response.status===204) return undefined as T
  return response.json() as Promise<T>
}
export const dashboardApi = {
  async login(token:string) { const session=await request<{csrfToken:string}>('/api/session',{method:'POST',body:JSON.stringify({token})}); csrfToken=session.csrfToken },
  async session() { const session=await request<{csrfToken:string}>('/api/session'); csrfToken=session.csrfToken },
  async logout() { await request<void>('/api/session',{method:'DELETE'}); csrfToken='' },
  dashboard() { return request<DashboardData>('/api/v1/dashboard') },
}
