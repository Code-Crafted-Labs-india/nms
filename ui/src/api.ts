export type MetricPoint = { time: string; device: string; value: number }
export type Device = { id:number; hostname:string; ipAddress:string; type:string; location:string; status:'online'|'offline'; rttMs:number|null; lastUpdate:string|null }
export type NetworkEvent = { time:string; device:string; severity:'INFO'|'WARNING'|'CRITICAL'; type:string; message:string }
export type TopologyEdge = { id:string; source:string; target:string; sourcePort:string; targetPort:string }
export type DashboardData = { generatedAt:string; overview:{ monitored:number; online:number; offline:number; openAlerts:number }; devices:Device[]; events:NetworkEvent[]; latency:MetricPoint[]; cpu:MetricPoint[]; memory:MetricPoint[]; topology:TopologyEdge[] }

// Inventory types — align with handlers/inventory.go
export type InventoryDevice = {
  id: number; hostname: string; ipAddress: string; deviceType: string; location: string
  isMonitored: boolean; snmpVersion: string; createdAt: string
  status: 'online'|'offline'|'unknown'
  rttMs: number|null; packetLoss: number|null; openAlarms: number; lastPoll: string|null
  upInterfaces: number; downInterfaces: number
}
export type InventoryResponse = { devices: InventoryDevice[]; total: number; page: number; pageSize: number; totalPages: number }

export type DeviceAlarm = {
  id: number; time: string; severity: string; eventType: string; message: string
  resolved: boolean; resolvedAt: string|null
}
export type DeviceInterface = {
  id: number; ifIndex: number; ifDescr: string; speedBps: number|null
  operStatus: number; adminStatus: number
  rxBytesDelta: number|null; txBytesDelta: number|null
  rxErrors: number|null; txErrors: number|null; rxDiscards: number|null; txDiscards: number|null
  opticalRxDbm: number|null; opticalTxDbm: number|null; lastSeen: string|null
}
export type DeviceDetail = {
  id: number; hostname: string; ipAddress: string; deviceType: string; location: string
  isMonitored: boolean; snmpVersion: string; createdAt: string
  status: string; rttMs: number|null; packetLoss: number|null; lastPoll: string|null
  cpuUtilization: number|null; memoryUtilization: number|null
  openAlarmCount: number; alarms: DeviceAlarm[]; interfaces: DeviceInterface[]
}

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
  inventory(params: Record<string, string|number> = {}) {
    const qs = new URLSearchParams(Object.entries(params).map(([k,v])=>[k,String(v)])).toString()
    return request<InventoryResponse>(`/api/v1/devices${qs ? '?'+qs : ''}`)
  },
  deviceDetail(id: number) { return request<DeviceDetail>(`/api/v1/devices/${id}`) },
  resolveAlarm(id: number) { return request<void>(`/api/v1/alarms/${id}/resolve`, { method: 'POST' }) },
}
