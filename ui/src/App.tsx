import { useCallback, useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import {
  Activity, AlertTriangle, Bell, CheckCircle, ChevronLeft, ChevronRight, ClipboardList,
  CircleCheck, Cpu, Filter, LayoutDashboard, LockKeyhole, LogOut,
  MemoryStick, Network, Plus, Radio, RefreshCw, Search,
  Server, ShieldCheck, Wifi, WifiOff, X, Zap,
  Pencil, Trash2,
} from 'lucide-react'
import {
  ApiError, dashboardApi,
  type DashboardData, type MetricPoint,
  type InventoryDevice, type DeviceDetail, type DeviceAlarm, type DeviceInterface,
  type DeviceModel, type DevicePayload, type AuditEvent,
} from './api'

type View = 'overview' | 'topology' | 'alerts' | 'inventory' | 'audit'
const emptyDashboard: DashboardData = {
  generatedAt: '', overview: { monitored: 0, online: 0, offline: 0, openAlerts: 0 },
  devices: [], events: [], latency: [], cpu: [], memory: [], topology: [],
}

// ────────────────────────── LOGIN ──────────────────────────
function Login({ onAuthenticated }: { onAuthenticated: () => void }) {
  const [token, setToken] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try { await dashboardApi.login(token); setToken(''); onAuthenticated() }
    catch { setError('Access denied. Verify the operator credential and try again.') }
    finally { setBusy(false) }
  }
  return <main className="grid min-h-screen place-items-center bg-[#07100f] px-6 text-slate-100">
    <div className="absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(36,211,164,.12),transparent_42%)]" />
    <section className="relative w-full max-w-md rounded-2xl border border-white/10 bg-[#0c1716]/95 p-8 shadow-2xl shadow-black/40">
      <div className="mb-8 flex items-center gap-3"><img src="/ccl-logo-dark.png" alt="Code Crafted Labs" className="size-12 rounded-full object-cover"/><div><p className="text-xs font-semibold uppercase tracking-[.18em] text-[#d6ff8a]">Code Crafted Labs</p><h1 className="text-xl font-semibold">NetPulse operations</h1></div></div>
      <div className="mb-6 rounded-lg border border-white/8 bg-white/[.025] p-4 text-sm leading-6 text-slate-400"><div className="mb-1 flex items-center gap-2 font-medium text-slate-200"><LockKeyhole size={15} /> Restricted system</div>Authorized personnel only. Activity is subject to monitoring and audit.</div>
      <form onSubmit={submit}><label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-slate-400" htmlFor="access-token">Operator credential</label><input id="access-token" type="password" autoComplete="current-password" required minLength={32} value={token} onChange={e => setToken(e.target.value)} className="w-full rounded-lg border border-white/10 bg-black/20 px-4 py-3 text-sm outline-none transition focus:border-emerald-400/60 focus:ring-2 focus:ring-emerald-400/10" />{error && <p role="alert" className="mt-3 text-sm text-rose-300">{error}</p>}<button disabled={busy} className="mt-5 flex w-full items-center justify-center gap-2 rounded-lg bg-emerald-400 px-4 py-3 text-sm font-bold text-[#06100e] transition hover:bg-emerald-300 disabled:opacity-50">{busy ? <RefreshCw className="animate-spin" size={16} /> : <LockKeyhole size={16} />} Authenticate</button></form>
      <p className="mt-6 text-center text-xs text-slate-600">Credentials are exchanged for an HttpOnly session and are never stored by this client.</p>
    </section>
  </main>
}

// ────────────────────────── SPARKLINE ──────────────────────────
function Sparkline({ points, color = '#34d399' }: { points: MetricPoint[], color?: string }) {
  const values = points.slice(-36).map(point => point.value)
  if (values.length < 2) return <div className="grid h-28 place-items-center text-xs text-slate-600">Awaiting telemetry</div>
  const min = Math.min(...values), max = Math.max(...values), range = Math.max(max - min, 1)
  const line = values.map((value, index) => `${(index / (values.length - 1)) * 300},${100 - ((value - min) / range) * 80}`).join(' ')
  return <svg viewBox="0 0 300 110" className="h-28 w-full overflow-visible" role="img" aria-label="Recent metric trend">{[20, 60, 100].map(y => <line key={y} x1="0" x2="300" y1={y} y2={y} stroke="rgba(148,163,184,.09)" />)}<polyline points={line} fill="none" stroke={color} strokeWidth="2.2" strokeLinejoin="round" strokeLinecap="round" /></svg>
}

// ────────────────────────── STAT CARD ──────────────────────────
function Stat({ label, value, detail, icon, tone = 'emerald' }: { label: string, value: number, detail: string, icon: React.ReactNode, tone?: 'emerald'|'rose'|'amber'|'sky' }) {
  const tones = { emerald:'text-emerald-300 bg-emerald-400/10 border-emerald-400/20', rose:'text-rose-300 bg-rose-400/10 border-rose-400/20', amber:'text-amber-300 bg-amber-400/10 border-amber-400/20', sky:'text-sky-300 bg-sky-400/10 border-sky-400/20' }
  return <article className="panel p-5"><div className="flex items-start justify-between"><div><p className="label">{label}</p><p className="mt-3 text-3xl font-semibold tracking-tight text-white">{value}</p></div><div className={`grid size-10 place-items-center rounded-lg border ${tones[tone]}`}>{icon}</div></div><p className="mt-4 text-xs text-slate-500">{detail}</p></article>
}

function MetricCard({ title, icon, points, color, suffix }: { title:string, icon:React.ReactNode, points:MetricPoint[], color:string, suffix:string }) {
  const last = points.at(-1)?.value
  return <article className="panel p-5"><header className="flex items-center justify-between"><div className="flex items-center gap-2 text-sm font-semibold text-slate-200">{icon}{title}</div><div className="text-lg font-semibold text-white">{last == null ? '—' : `${last.toFixed(1)}${suffix}`}</div></header><div className="mt-2"><Sparkline points={points} color={color}/></div></article>
}

// ────────────────────────── DEVICE TABLE ──────────────────────────
function DeviceTable({ devices }: { devices: DashboardData['devices'] }) {
  return <section className="panel mt-4 overflow-hidden"><header className="flex items-center justify-between border-b border-white/7 px-5 py-4"><div><p className="label">Asset status</p><h2 className="mt-1 font-semibold text-white">Managed infrastructure</h2></div><Network className="text-slate-500" size={20}/></header><div className="overflow-x-auto"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-black/10 text-[11px] uppercase tracking-wider text-slate-500"><tr><th>Asset</th><th>Address</th><th>Role</th><th>Location</th><th>Latency</th><th>Status</th></tr></thead><tbody>{devices.map(device=><tr key={device.id} className="border-t border-white/[.055]"><td><div className="font-medium text-slate-200">{device.hostname}</div></td><td className="font-mono text-xs text-slate-400">{device.ipAddress}</td><td>{device.type}</td><td>{device.location}</td><td>{device.rttMs == null ? '—' : `${device.rttMs.toFixed(1)} ms`}</td><td><span className={`status ${device.status === 'online' ? 'status-up':'status-down'}`}><span className="size-1.5 rounded-full bg-current"/>{device.status}</span></td></tr>)}</tbody></table>{!devices.length && <div className="p-10 text-center text-sm text-slate-500">No managed assets returned by the telemetry service.</div>}</div></section>
}

// ────────────────────────── TOPOLOGY ──────────────────────────
function TopologyMap({ data }: { data: DashboardData }) {
  const [hoveredNode, setHoveredNode] = useState<string|null>(null)
  const nodes = useMemo(() => {
    const names = new Set(data.devices.map(device => device.hostname))
    data.topology.forEach(edge => { names.add(edge.source); names.add(edge.target) })
    const ordered = [...names].sort((left, right) => {
      const leftCore = data.devices.find(device => device.hostname === left)?.type.toLowerCase().includes('core') ? -1 : 0
      const rightCore = data.devices.find(device => device.hostname === right)?.type.toLowerCase().includes('core') ? -1 : 0
      return leftCore - rightCore || left.localeCompare(right)
    })
    const hasCore = Boolean(data.devices.find(device => device.hostname === ordered[0])?.type.toLowerCase().includes('core'))
    return ordered.map((name, index) => {
      const device = data.devices.find(item => item.hostname === name)
      if (index === 0 && hasCore) return { name, device, x: 450, y: 270 }
      const orbit = hasCore ? ordered.slice(1) : ordered
      const orbitIndex = hasCore ? index - 1 : index
      const angle = (Math.PI * 2 * orbitIndex / Math.max(orbit.length, 1)) - Math.PI / 2
      return { name, device, x: 450 + Math.cos(angle) * 310, y: 270 + Math.sin(angle) * 190 }
    })
  }, [data.devices, data.topology])
  const positions = new Map(nodes.map(node => [node.name, node]))
  const offline = data.devices.filter(device => device.status === 'offline')
  const statusColor = (status?: string) => status === 'online' ? '#34d399' : status === 'offline' ? '#fb7185' : '#64748b'

  return <section className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_280px]">
    <article className="panel overflow-hidden">
      <header className="flex flex-wrap items-center justify-between gap-3 border-b border-white/7 px-5 py-4"><div><p className="label">LLDP topology</p><h2 className="mt-1 font-semibold text-white">Device interconnection map</h2></div><div className="flex gap-4 text-xs"><span className="flex items-center gap-2 text-slate-400"><i className="size-2 rounded-full bg-emerald-400"/>Online</span><span className="flex items-center gap-2 text-slate-400"><i className="size-2 rounded-full bg-rose-400"/>Down</span><span className="flex items-center gap-2 text-slate-400"><i className="size-2 rounded-full bg-slate-500"/>Unknown</span></div></header>
      <div className="overflow-x-auto bg-[radial-gradient(circle_at_center,rgba(52,211,153,.055),transparent_48%)] p-3">
        <svg viewBox="0 0 900 540" className="min-h-[480px] min-w-[760px] w-full" role="img" aria-label="Network topology showing device links and operational status">
          <defs><filter id="nodeGlow"><feGaussianBlur stdDeviation="5" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter></defs>
          {data.topology.map(edge => {
            const source = positions.get(edge.source), target = positions.get(edge.target)
            if (!source || !target) return null
            const down = source.device?.status === 'offline' || target.device?.status === 'offline'
            return <g key={edge.id}><line x1={source.x} y1={source.y} x2={target.x} y2={target.y} stroke={down ? '#9f4251' : '#28584c'} strokeWidth={down ? 2.5 : 2} strokeDasharray={down ? '7 5' : undefined}/><circle cx={(source.x+target.x)/2} cy={(source.y+target.y)/2} r="3" fill={down ? '#fb7185' : '#34d399'}><title>{edge.sourcePort || 'port'} ↔ {edge.targetPort || 'port'}</title></circle></g>
          })}
          {nodes.map(node => {
            const color = statusColor(node.device?.status)
            const selected = hoveredNode === node.name
            return <g key={node.name} transform={`translate(${node.x} ${node.y})`} tabIndex={0} role="img"
              aria-label={`${node.name}, ${node.device?.status || 'unknown'}, ${node.device?.ipAddress || 'unmanaged'}`}
              onMouseEnter={() => setHoveredNode(node.name)} onMouseLeave={() => setHoveredNode(null)}
              onFocus={() => setHoveredNode(node.name)} onBlur={() => setHoveredNode(null)}
              className="cursor-default outline-none">
              <circle r="38" fill={color} opacity=".08" filter="url(#nodeGlow)"/>
              <rect x="-72" y="-31" width="144" height="62" rx="12" fill="#0d1917" stroke={selected ? '#d6ff8a' : color} strokeWidth={selected ? 2.5 : 2}/>
              <circle cx="-50" cy="-8" r="7" fill={color}/>
              <text x="-36" y="-3" fill="#e2e8f0" fontSize="13" fontWeight="600">{node.name.length > 17 ? `${node.name.slice(0,16)}…` : node.name}</text>
              <text x="-50" y="17" fill="#64748b" fontSize="10">{node.device?.type || 'Unmanaged device'}</text>
              <title>{node.name} — {node.device?.status || 'unknown'}{node.device?.ipAddress ? ` — ${node.device.ipAddress}` : ''}</title>
            </g>
          })}
          {hoveredNode && (() => {
            const node = nodes.find(item => item.name === hoveredNode)
            if (!node) return null
            const device = node.device
            const x = Math.max(16, Math.min(644, node.x - 120))
            const y = node.y > 180 ? node.y - 145 : node.y + 45
            const color = statusColor(device?.status)
            const truncate = (value:string, length:number) => value.length > length ? `${value.slice(0,length-1)}…` : value
            return <g pointerEvents="none" aria-hidden="true">
              <rect x={x} y={y} width="240" height="128" rx="12" fill="#101a18" stroke="#d6ff8a" strokeOpacity=".72" strokeWidth="1.5" filter="url(#nodeGlow)"/>
              <text x={x+14} y={y+22} fill="#f1f5f9" fontSize="13" fontWeight="700">{truncate(node.name,28)}</text>
              <circle cx={x+18} cy={y+42} r="4" fill={color}/>
              <text x={x+29} y={y+46} fill={color} fontSize="10" fontWeight="700">{(device?.status || 'unknown').toUpperCase()}</text>
              <text x={x+14} y={y+66} fill="#94a3b8" fontSize="10">IP · {device?.ipAddress || 'Not in managed inventory'}</text>
              <text x={x+14} y={y+85} fill="#94a3b8" fontSize="10">Type · {truncate(device?.type || 'Unmanaged device',30)}</text>
              <text x={x+14} y={y+104} fill="#94a3b8" fontSize="10">Location · {truncate(device?.location || '—',28)}</text>
              <text x={x+14} y={y+120} fill="#64748b" fontSize="9">{device?.rttMs != null ? `RTT ${device.rttMs.toFixed(1)} ms` : 'RTT —'}  ·  {device?.lastUpdate ? formatTime(device.lastUpdate) : 'No recent telemetry'}</text>
            </g>
          })()}
        </svg>
        {!data.topology.length && <div className="mx-auto max-w-xl pb-5 text-center"><p className="text-sm font-medium text-slate-300">No live LLDP neighbors received</p><p className="mt-1 text-xs leading-5 text-slate-500">Connections appear after monitored devices report neighbors through LLDP-MIB over SNMP. Enable LLDP and allow LLDP-MIB reads for the configured Telegraf SNMP targets.</p></div>}
      </div>
    </article>
    <aside className="space-y-4"><article className="panel p-5"><p className="label">Topology summary</p><div className="mt-4 space-y-3"><div className="flex justify-between text-sm text-slate-400"><span>Visible nodes</span><strong className="text-white">{nodes.length}</strong></div><div className="h-px bg-white/7"/><div className="flex justify-between text-sm text-slate-400"><span>Discovered links</span><strong className="text-white">{data.topology.length}</strong></div><div className="h-px bg-white/7"/><div className="flex justify-between text-sm text-slate-400"><span>Down devices</span><strong className={offline.length ? 'text-rose-300':'text-emerald-300'}>{offline.length}</strong></div></div></article><article className="panel overflow-hidden"><header className="border-b border-white/7 p-4"><p className="label">Service impact</p></header>{offline.length ? <div className="divide-y divide-white/[.06]">{offline.map(device=><div key={device.id} className="flex items-center gap-3 p-4"><span className="size-2 rounded-full bg-rose-400 shadow-[0_0_8px_#fb7185]"/><div><p className="text-sm font-medium text-slate-200">{device.hostname}</p><p className="text-xs text-slate-500">{device.ipAddress}</p></div></div>)}</div> : <div className="grid place-items-center gap-2 p-8 text-center text-sm text-slate-500"><CircleCheck className="text-emerald-400" size={24}/>All visible devices operational</div>}</article></aside>
  </section>
}

// ────────────────────────── OVERVIEW ──────────────────────────
function Overview({ data }: { data: DashboardData }) {
  const latency = useMemo(() => data.latency.slice(-80), [data.latency])
  const health = data.overview.monitored ? Math.round(data.overview.online / data.overview.monitored * 100) : 0
  return <><section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><Stat label="Monitored assets" value={data.overview.monitored} detail="Inventory under active supervision" icon={<Server size={19}/>} tone="sky" /><Stat label="Operational" value={data.overview.online} detail={`${health}% fleet availability`} icon={<Wifi size={19}/>} /><Stat label="Unreachable" value={data.overview.offline} detail="No response within five minutes" icon={<WifiOff size={19}/>} tone="rose" /><Stat label="Open alerts" value={data.overview.openAlerts} detail="Unresolved operational events" icon={<Bell size={19}/>} tone="amber" /></section>
    <section className="mt-4 grid gap-4 xl:grid-cols-[1.6fr_1fr]"><article className="panel p-5"><header className="flex items-center justify-between"><div><p className="label">Network latency</p><h2 className="mt-1 font-semibold text-white">Response time · last 30 minutes</h2></div><div className="flex items-center gap-2 text-xs text-emerald-300"><span className="size-2 rounded-full bg-emerald-400"/> Live</div></header><div className="mt-5"><Sparkline points={latency}/></div></article><article className="panel p-5"><p className="label">Fleet posture</p><div className="mt-5 flex items-center gap-6"><div className="relative grid size-28 shrink-0 place-items-center rounded-full" style={{background:`conic-gradient(#34d399 ${health}%, #25302e 0)`}}><div className="grid size-20 place-items-center rounded-full bg-[#0d1817]"><span className="text-2xl font-semibold text-white">{health}%</span></div></div><div className="w-full space-y-3 text-sm"><div className="flex justify-between text-slate-400"><span>Operational</span><strong className="text-emerald-300">{data.overview.online}</strong></div><div className="h-px bg-white/8"/><div className="flex justify-between text-slate-400"><span>Unreachable</span><strong className="text-rose-300">{data.overview.offline}</strong></div><div className="h-px bg-white/8"/><div className="flex justify-between text-slate-400"><span>Unclassified</span><strong className="text-slate-200">{Math.max(data.overview.monitored-data.overview.online-data.overview.offline,0)}</strong></div></div></div></article></section>
    <section className="mt-4 grid gap-4 xl:grid-cols-2"><MetricCard title="CPU utilization" icon={<Cpu size={17}/>} points={data.cpu} color="#38bdf8" suffix="%" /><MetricCard title="Memory utilization" icon={<MemoryStick size={17}/>} points={data.memory} color="#a78bfa" suffix="%" /></section><DeviceTable devices={data.devices.slice(0, 6)} /></>
}

// ────────────────────────── ALERTS ──────────────────────────
function Alerts({ events }: { events: DashboardData['events'] }) {
  return <section className="panel overflow-hidden"><header className="border-b border-white/7 px-5 py-5"><p className="label">Event stream</p><h2 className="mt-1 text-lg font-semibold text-white">Active alerts</h2></header><div className="divide-y divide-white/[.06]">{events.map((event,index)=><article key={`${event.time}-${index}`} className="flex gap-4 p-5"><div className={`mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg ${event.severity==='CRITICAL'?'bg-rose-400/10 text-rose-300':'bg-amber-400/10 text-amber-300'}`}><AlertTriangle size={17}/></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><strong className="text-sm text-slate-200">{event.type}</strong><span className="text-xs text-slate-500">{event.device}</span></div><p className="mt-1 text-sm text-slate-400">{event.message}</p><time className="mt-2 block text-xs text-slate-600">{formatTime(event.time)}</time></div></article>)}{!events.length&&<div className="grid place-items-center gap-3 p-16 text-sm text-slate-500"><CircleCheck size={28} className="text-emerald-400"/>No unresolved alerts</div>}</div></section>
}

function AuditView() {
  const [events, setEvents] = useState<AuditEvent[]>([])
  const [page, setPage] = useState(1)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(true)
  useEffect(() => {
    let active = true
    setLoading(true); setError('')
    dashboardApi.audit(page).then(result => { if (active) setEvents(result.events) })
      .catch(() => { if (active) setError('Audit activity could not be loaded.') })
      .finally(() => { if (active) setLoading(false) })
    return () => { active = false }
  }, [page])
  return <section className="panel overflow-hidden">
    <header className="flex flex-wrap items-center justify-between gap-3 border-b border-white/7 px-5 py-5">
      <div><p className="label">Administrative activity</p><h2 className="mt-1 text-lg font-semibold text-white">Audit events</h2>
        <p className="mt-1 text-xs text-slate-500">Shared bootstrap login is recorded as bootstrap-admin.</p></div>
      <div className="flex items-center gap-2">
        <button disabled={page<=1||loading} onClick={()=>setPage(p=>Math.max(1,p-1))} className="rounded-lg border border-white/10 px-3 py-2 text-xs disabled:opacity-40">Previous</button>
        <span className="text-xs text-slate-500">Page {page}</span>
        <button disabled={events.length<50||loading} onClick={()=>setPage(p=>p+1)} className="rounded-lg border border-white/10 px-3 py-2 text-xs disabled:opacity-40">Next</button>
      </div>
    </header>
    {error&&<p role="alert" className="m-5 rounded-lg border border-rose-400/20 bg-rose-400/5 p-3 text-sm text-rose-300">{error}</p>}
    {loading?<p className="p-8 text-sm text-slate-500">Loading audit events…</p>:<div className="overflow-x-auto">
      <table className="w-full min-w-[760px] text-left text-sm"><thead className="bg-black/10 text-[10px] uppercase tracking-wider text-slate-500"><tr>
        <th className="px-5 py-3">Time</th><th className="px-5 py-3">Actor</th><th className="px-5 py-3">Action</th><th className="px-5 py-3">Request</th><th className="px-5 py-3">Result</th>
      </tr></thead><tbody>{events.map(event=><tr key={event.id} className="border-t border-white/[.055]">
        <td className="whitespace-nowrap px-5 py-3 text-xs text-slate-500">{formatTime(event.occurred_at)}</td>
        <td className="px-5 py-3 text-xs text-slate-300">{event.actor}</td>
        <td className="px-5 py-3 text-xs text-slate-300">{event.action}</td>
        <td className="px-5 py-3 font-mono text-[11px] text-slate-500">{event.method} {event.path}</td>
        <td className="px-5 py-3"><span className={event.outcome==='success'?'text-emerald-300':'text-rose-300'}>{event.outcome} · {event.status_code}</span></td>
      </tr>)}</tbody></table>
      {!events.length&&!error&&<div className="grid place-items-center gap-3 p-12 text-sm text-slate-500"><ClipboardList size={25}/>No audit events recorded yet</div>}
    </div>}
  </section>
}

// ────────────────────────── DEVICE DETAIL PANEL ──────────────────────────
function StatusBadge({ status }: { status: string }) {
  const map: Record<string, string> = {
    online: 'bg-emerald-400/10 text-emerald-300 border-emerald-400/20',
    offline: 'bg-rose-400/10 text-rose-300 border-rose-400/20',
    unknown: 'bg-slate-400/10 text-slate-400 border-slate-400/20',
  }
  return <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium ${map[status] ?? map.unknown}`}><span className="size-1.5 rounded-full bg-current"/>{status}</span>
}

function AlarmRow({ alarm, onResolve }: { alarm: DeviceAlarm; onResolve: (id: number) => void }) {
  const sevColor = alarm.severity === 'CRITICAL' ? 'text-rose-300' : alarm.severity === 'WARNING' ? 'text-amber-300' : 'text-slate-400'
  return <div className="flex items-start gap-3 border-b border-white/[.055] py-3 last:border-0">
    <AlertTriangle size={15} className={`mt-0.5 shrink-0 ${sevColor}`} />
    <div className="min-w-0 flex-1">
      <div className="flex flex-wrap items-center gap-2">
        <span className={`text-xs font-semibold ${sevColor}`}>{alarm.severity}</span>
        <span className="text-xs text-slate-500">{alarm.eventType}</span>
        {alarm.resolved && <span className="rounded bg-emerald-400/10 px-1.5 py-0.5 text-[10px] text-emerald-300">resolved</span>}
      </div>
      <p className="mt-1 text-xs text-slate-400">{alarm.message}</p>
      <time className="mt-1 block text-[10px] text-slate-600">{formatTime(alarm.time)}</time>
    </div>
    {!alarm.resolved && (
      <button onClick={() => onResolve(alarm.id)} title="Resolve alarm"
        className="mt-0.5 grid size-7 shrink-0 place-items-center rounded-lg border border-emerald-400/20 bg-emerald-400/10 text-emerald-300 hover:bg-emerald-400/20 transition">
        <CheckCircle size={13}/>
      </button>
    )}
  </div>
}

function InterfaceRow({ iface }: { iface: DeviceInterface }) {
  const operColor = iface.operStatus === 1 ? 'text-emerald-400' : 'text-rose-400'
  const speedStr = iface.speedBps ? (iface.speedBps >= 1e9 ? `${(iface.speedBps/1e9).toFixed(0)}G` : `${(iface.speedBps/1e6).toFixed(0)}M`) : '—'
  return <tr className="border-t border-white/[.05] text-xs">
    <td className="py-2 pr-3 font-mono text-slate-300">{iface.ifDescr}</td>
    <td><span className={operColor}>{iface.operStatus === 1 ? 'UP' : iface.operStatus === 2 ? 'DOWN' : '?'}</span></td>
    <td className="text-slate-500">{speedStr}</td>
    <td className="text-slate-500">{iface.rxBytesDelta != null ? `${(iface.rxBytesDelta/1024).toFixed(0)} KB` : '—'}</td>
    <td className="text-slate-500">{iface.txBytesDelta != null ? `${(iface.txBytesDelta/1024).toFixed(0)} KB` : '—'}</td>
    <td className="text-rose-400/80">{(iface.rxErrors ?? 0) + (iface.txErrors ?? 0) > 0 ? (iface.rxErrors ?? 0) + (iface.txErrors ?? 0) : '—'}</td>
  </tr>
}

function DeviceDetailPanel({ deviceId, onClose }: { deviceId: number; onClose: () => void }) {
  const [detail, setDetail] = useState<DeviceDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [tab, setTab] = useState<'overview'|'interfaces'|'alarms'>('overview')

  const load = useCallback(async () => {
    setLoading(true)
    try { setDetail(await dashboardApi.deviceDetail(deviceId)) } finally { setLoading(false) }
  }, [deviceId])

  useEffect(() => { load() }, [load])

  async function resolveAlarm(id: number) {
    await dashboardApi.resolveAlarm(id)
    load()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-end bg-black/50 backdrop-blur-sm" onClick={onClose}>
      <aside className="h-full w-full max-w-2xl overflow-y-auto bg-[#0c1716] shadow-2xl"
        onClick={e => e.stopPropagation()}>
        <header className="sticky top-0 z-10 flex items-center justify-between border-b border-white/8 bg-[#0c1716]/95 px-6 py-4 backdrop-blur">
          <div>
            <p className="text-xs font-semibold uppercase tracking-widest text-emerald-400">Device detail</p>
            <h2 className="text-lg font-semibold text-white">{detail?.hostname ?? '…'}</h2>
          </div>
          <button onClick={onClose} className="grid size-9 place-items-center rounded-lg border border-white/10 text-slate-500 hover:text-white"><X size={18}/></button>
        </header>

        {loading && <div className="grid h-64 place-items-center"><RefreshCw className="animate-spin text-emerald-400" size={24}/></div>}

        {!loading && detail && <>
          {/* Identity row */}
          <div className="grid grid-cols-2 gap-3 border-b border-white/8 p-6 sm:grid-cols-4">
            {[
              { label: 'IP address', value: detail.ipAddress },
              { label: 'Type', value: detail.deviceType },
              { label: 'Location', value: detail.location },
              { label: 'SNMP', value: detail.snmpVersion },
            ].map(item => (
              <div key={item.label}>
                <p className="text-[10px] uppercase tracking-widest text-slate-600">{item.label}</p>
                <p className="mt-1 text-sm text-slate-200">{item.value}</p>
              </div>
            ))}
          </div>

          {/* Status bar */}
          <div className="flex flex-wrap items-center gap-4 border-b border-white/8 px-6 py-4">
            <StatusBadge status={detail.status} />
            {detail.rttMs != null && <span className="text-sm text-slate-400">RTT <strong className="text-white">{detail.rttMs.toFixed(1)} ms</strong></span>}
            {detail.packetLoss != null && detail.packetLoss > 0 && <span className="text-sm text-rose-300">Loss <strong>{detail.packetLoss.toFixed(1)}%</strong></span>}
            {detail.cpuUtilization != null && <span className="flex items-center gap-1 text-sm text-slate-400"><Cpu size={13}/><strong className="text-white">{detail.cpuUtilization.toFixed(1)}%</strong> CPU</span>}
            {detail.memoryUtilization != null && <span className="flex items-center gap-1 text-sm text-slate-400"><MemoryStick size={13}/><strong className="text-white">{detail.memoryUtilization.toFixed(1)}%</strong> Mem</span>}
            {detail.openAlarmCount > 0 && <span className="flex items-center gap-1 text-sm text-amber-300"><AlertTriangle size={13}/>{detail.openAlarmCount} open alarm{detail.openAlarmCount !== 1 && 's'}</span>}
          </div>

          {/* Tabs */}
          <div className="flex border-b border-white/8">
            {(['overview','interfaces','alarms'] as const).map(t => (
              <button key={t} onClick={() => setTab(t)}
                className={`px-5 py-3 text-xs font-semibold uppercase tracking-wider transition ${tab === t ? 'border-b-2 border-emerald-400 text-emerald-300' : 'text-slate-500 hover:text-slate-300'}`}>
                {t}{t === 'alarms' && detail.alarms.length > 0 ? ` (${detail.alarms.length})` : ''}
              </button>
            ))}
          </div>

          {/* Tab body */}
          <div className="p-6">
            {tab === 'overview' && (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-3">
                  <div className="rounded-xl border border-white/8 bg-white/[.02] p-4">
                    <p className="text-[10px] uppercase tracking-widest text-slate-600">Last poll</p>
                    <p className="mt-1 text-sm text-slate-200">{detail.lastPoll ? formatTime(detail.lastPoll) : '—'}</p>
                  </div>
                  <div className="rounded-xl border border-white/8 bg-white/[.02] p-4">
                    <p className="text-[10px] uppercase tracking-widest text-slate-600">Discovered</p>
                    <p className="mt-1 text-sm text-slate-200">{formatTime(detail.createdAt)}</p>
                  </div>
                </div>
                <div className="rounded-xl border border-white/8 bg-white/[.02] p-4">
                  <p className="mb-3 text-[10px] uppercase tracking-widest text-slate-600">Interface summary</p>
                  <div className="flex gap-6 text-sm">
                    <div><p className="text-slate-500">Up</p><p className="text-2xl font-semibold text-emerald-300">{detail.interfaces.filter(i=>i.operStatus===1).length}</p></div>
                    <div><p className="text-slate-500">Down</p><p className="text-2xl font-semibold text-rose-300">{detail.interfaces.filter(i=>i.operStatus===2).length}</p></div>
                    <div><p className="text-slate-500">Total</p><p className="text-2xl font-semibold text-white">{detail.interfaces.length}</p></div>
                  </div>
                </div>
              </div>
            )}

            {tab === 'interfaces' && (
              <div className="overflow-x-auto">
                <table className="w-full min-w-[500px] text-left">
                  <thead className="text-[10px] uppercase tracking-wider text-slate-600">
                    <tr><th className="pb-2 pr-3">Interface</th><th className="pb-2">State</th><th className="pb-2">Speed</th><th className="pb-2">RX</th><th className="pb-2">TX</th><th className="pb-2">Errors</th></tr>
                  </thead>
                  <tbody>{detail.interfaces.map(iface => <InterfaceRow key={iface.id} iface={iface}/>)}</tbody>
                </table>
                {!detail.interfaces.length && <p className="py-8 text-center text-sm text-slate-500">No interface data available.</p>}
              </div>
            )}

            {tab === 'alarms' && (
              <div>
                {detail.alarms.map(alarm => <AlarmRow key={alarm.id} alarm={alarm} onResolve={resolveAlarm}/>)}
                {!detail.alarms.length && <div className="grid place-items-center gap-3 py-12 text-sm text-slate-500"><CircleCheck size={24} className="text-emerald-400"/>No alarm history for this device.</div>}
              </div>
            )}
          </div>
        </>}
      </aside>
    </div>
  )
}

// ────────────────────────── INVENTORY VIEW ──────────────────────────
type DeviceDraft = {
  hostname: string; ipAddress: string; deviceType: string; location: string
  snmpVersion: string; snmpCommunity: string; isMonitored: boolean
}

function DeviceEditor({ device, models, onClose, onSave }: {
  device: InventoryDevice|null; models: DeviceModel[]; onClose: () => void
  onSave: (payload: DevicePayload) => Promise<void>
}) {
  const [draft, setDraft] = useState<DeviceDraft>({
    hostname: device?.hostname ?? '', ipAddress: device?.ipAddress ?? '',
    deviceType: device?.deviceType ?? models[0]?.modelName ?? '', location: device?.location ?? '',
    snmpVersion: device?.snmpVersion ?? 'v2c', snmpCommunity: '', isMonitored: device?.isMonitored ?? true,
  })
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const update = <K extends keyof DeviceDraft>(key: K, value: DeviceDraft[K]) => setDraft(current => ({ ...current, [key]: value }))
  async function submit(event: FormEvent) {
    event.preventDefault(); setBusy(true); setError('')
    try {
      await onSave({ hostname:draft.hostname.trim(), ip_address:draft.ipAddress.trim(), device_type:draft.deviceType,
        location:draft.location.trim(), snmp_community:draft.snmpCommunity, snmp_version:draft.snmpVersion,
        is_monitored:draft.isMonitored })
    } catch (err) { setError(err instanceof Error ? err.message : 'Unable to save this device.') }
    finally { setBusy(false) }
  }
  const field = 'mt-1.5 w-full rounded-lg border border-white/10 bg-[#080f0e] px-3 py-2.5 text-sm text-slate-100 outline-none transition placeholder:text-slate-600 focus:border-[#d6ff8a]/60 focus:ring-2 focus:ring-[#d6ff8a]/10'
  const label = 'block text-xs font-semibold text-slate-400'
  return <div className="fixed inset-0 z-[60] grid place-items-center overflow-y-auto bg-black/70 p-4 backdrop-blur-sm" onMouseDown={event=>{if(event.target===event.currentTarget)onClose()}}>
    <section role="dialog" aria-modal="true" aria-labelledby="device-form-title" className="my-auto w-full max-w-2xl overflow-hidden rounded-2xl border border-white/10 bg-[#0c1210] shadow-2xl shadow-black/60">
      <header className="flex items-center justify-between border-b border-white/8 px-6 py-5"><div className="flex items-center gap-3"><img src="/ccl-logo-dark.png" alt="" className="size-10 rounded-full object-cover"/><div><p className="text-[10px] font-bold uppercase tracking-[.2em] text-[#d6ff8a]">Code Crafted Labs · NetPulse</p><h2 id="device-form-title" className="mt-1 text-lg font-semibold text-white">{device ? 'Edit device' : 'Add a device'}</h2></div></div><button type="button" onClick={onClose} aria-label="Close" className="grid size-9 place-items-center rounded-lg border border-white/10 text-slate-500 hover:text-white"><X size={17}/></button></header>
      <form onSubmit={submit} className="space-y-6 p-6">
        <div><p className="label">Device identity</p><div className="mt-3 grid gap-4 sm:grid-cols-2">
          <label className={label}>Hostname <span className="text-[#d6ff8a]">*</span><input className={field} value={draft.hostname} onChange={e=>update('hostname',e.target.value)} required maxLength={255} placeholder="edge-router-01" autoFocus /></label>
          <label className={label}>Management IP <span className="text-[#d6ff8a]">*</span><input className={field} type="text" inputMode="decimal" value={draft.ipAddress} onChange={e=>update('ipAddress',e.target.value)} required placeholder="192.168.10.10" pattern="[0-9a-fA-F:.]+(/[0-9]{1,3})?" title="Enter an IPv4 or IPv6 address, optionally with a CIDR prefix" /></label>
          <label className={label}>Hardware model <span className="text-[#d6ff8a]">*</span><select className={field} value={draft.deviceType} onChange={e=>update('deviceType',e.target.value)} required><option value="" disabled>Select supported model</option>{device&&!models.some(model=>model.modelName===device.deviceType)&&<option value={device.deviceType}>{device.deviceType} · current</option>}{models.map(model=><option key={model.modelName} value={model.modelName}>{model.modelName}</option>)}</select>{!models.length&&<span className="mt-1 block text-[11px] text-amber-300">Hardware catalog unavailable</span>}</label>
          <label className={label}>Site / location<input className={field} value={draft.location} onChange={e=>update('location',e.target.value)} maxLength={255} placeholder="Data center · Rack 04" /></label>
        </div></div>
        <div className="border-t border-white/8 pt-5"><p className="label">Monitoring and access</p><div className="mt-3 grid gap-4 sm:grid-cols-2">
          <label className={label}>SNMP version<select className={field} value={draft.snmpVersion} onChange={e=>update('snmpVersion',e.target.value)}><option value="v2c">SNMPv2c · migration / lab</option>{device?.snmpVersion==='v3'&&<option value="v3">SNMPv3 · current</option>}</select>{!device&&<span className="mt-1 block text-[11px] text-slate-600">SNMPv3 onboarding is available when credential profiles are added.</span>}</label>
          {draft.snmpVersion==='v2c'&&<label className={label}>SNMP community{device&&<span className="ml-1 font-normal text-slate-600">(leave blank to keep current)</span>}<input className={field} type="password" autoComplete="new-password" value={draft.snmpCommunity} onChange={e=>update('snmpCommunity',e.target.value)} placeholder={device?'Stored value is hidden':'Defaults to public'} /></label>}
        </div>
        <label className="mt-4 flex cursor-pointer items-start gap-3 rounded-xl border border-white/8 bg-white/[.02] p-4"><input type="checkbox" checked={draft.isMonitored} onChange={e=>update('isMonitored',e.target.checked)} className="mt-0.5 accent-[#d6ff8a]"/><span><strong className="block text-sm font-medium text-slate-200">Monitor this device</strong><span className="mt-1 block text-xs text-slate-500">Include this device in reachability monitoring and fleet health.</span></span></label>
        </div>
        {error&&<p role="alert" className="rounded-lg border border-rose-400/20 bg-rose-400/8 px-3 py-2 text-sm text-rose-300">{error}</p>}
        <footer className="flex flex-col-reverse gap-2 border-t border-white/8 pt-4 sm:flex-row sm:justify-end"><button type="button" onClick={onClose} className="rounded-lg border border-white/10 px-4 py-2.5 text-sm text-slate-400 hover:text-white">Cancel</button><button disabled={busy||!models.length} className="inline-flex items-center justify-center gap-2 rounded-lg bg-[#d6ff8a] px-4 py-2.5 text-sm font-bold text-[#10150b] transition hover:bg-[#e2ffad] disabled:opacity-50">{busy?<RefreshCw size={15} className="animate-spin"/>:<CheckCircle size={15}/>} {device?'Save changes':'Add device'}</button></footer>
      </form>
    </section>
  </div>
}

function InventoryView() {
  const [devices, setDevices] = useState<InventoryDevice[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [totalPages, setTotalPages] = useState(1)
  const [loading, setLoading] = useState(false)
  const [search, setSearch] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [typeFilter, setTypeFilter] = useState('')
  const [selectedId, setSelectedId] = useState<number|null>(null)
  const [models, setModels] = useState<DeviceModel[]>([])
  const [editorDevice, setEditorDevice] = useState<InventoryDevice|null|undefined>(undefined)
  const [deleteTarget, setDeleteTarget] = useState<InventoryDevice|null>(null)
  const [actionError, setActionError] = useState('')
  const [saving, setSaving] = useState(false)
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => { dashboardApi.deviceModels().then(setModels).catch(() => setModels([])) }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const params: Record<string, string|number> = { page, page_size: 50 }
      if (search) params.q = search
      if (statusFilter) params.status = statusFilter
      if (typeFilter) params.type = typeFilter
      const res = await dashboardApi.inventory(params)
      setDevices(res.devices ?? [])
      setTotal(res.total ?? 0)
      setTotalPages(res.totalPages ?? 1)
    } finally {
      setLoading(false)
    }
  }, [page, search, statusFilter, typeFilter, reloadKey])

  async function saveDevice(payload:DevicePayload) {
    if (editorDevice) await dashboardApi.updateDevice(editorDevice.id,payload)
    else await dashboardApi.createDevice(payload)
    setEditorDevice(undefined); setActionError(''); setReloadKey(key=>key+1)
  }

  async function deleteDevice() {
    if (!deleteTarget) return
    setSaving(true); setActionError('')
    try { await dashboardApi.deleteDevice(deleteTarget.id); setDeleteTarget(null); setSelectedId(null); setReloadKey(key=>key+1) }
    catch (err) { setActionError(err instanceof Error ? err.message : 'Unable to delete this device.') }
    finally { setSaving(false) }
  }

  useEffect(() => { load() }, [load])

  const statusColors: Record<string, string> = {
    online:  'text-emerald-300 border-emerald-400/30 bg-emerald-400/10',
    offline: 'text-rose-300 border-rose-400/30 bg-rose-400/10',
    unknown: 'text-slate-400 border-slate-600/30 bg-slate-400/10',
  }

  return (
    <div className="space-y-4">
      {/* Filters bar */}
      <div className="panel flex flex-wrap items-center gap-3 p-4">
        <div className="relative flex-1 min-w-48">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-600" size={14}/>
          <input value={search} onChange={e=>{setSearch(e.target.value);setPage(1)}}
            placeholder="Search hostname or IP…"
            className="w-full rounded-lg border border-white/8 bg-black/20 py-2 pl-9 pr-3 text-sm outline-none focus:border-emerald-400/40"/>
        </div>
        <div className="flex items-center gap-2">
          <Filter size={14} className="text-slate-500"/>
          <select value={statusFilter} onChange={e=>{setStatusFilter(e.target.value);setPage(1)}}
            className="rounded-lg border border-white/8 bg-[#0c1716] px-3 py-2 text-sm text-slate-300 outline-none focus:border-emerald-400/40">
            <option value="">All statuses</option>
            <option value="online">Online</option>
            <option value="offline">Offline</option>
            <option value="unknown">Unknown</option>
          </select>
          <input value={typeFilter} onChange={e=>{setTypeFilter(e.target.value);setPage(1)}}
            placeholder="Filter by type…"
            className="rounded-lg border border-white/8 bg-black/20 px-3 py-2 text-sm text-slate-300 outline-none focus:border-emerald-400/40 w-36"/>
        </div>
        <div className="ml-auto flex items-center gap-2 text-xs text-slate-500">
          {loading ? <RefreshCw size={13} className="animate-spin"/> : null}
          <span>{total} device{total !== 1 ? 's' : ''}</span>
        </div>
        <button onClick={()=>{setActionError('');setSelectedId(null);setEditorDevice(null)}} className="inline-flex items-center gap-2 rounded-lg bg-[#d6ff8a] px-3.5 py-2 text-xs font-bold text-[#10150b] transition hover:bg-[#e2ffad]"><Plus size={15}/> Add device</button>
      </div>
      {actionError&&!deleteTarget&&<div role="alert" className="flex items-center gap-2 rounded-lg border border-rose-400/20 bg-rose-400/8 px-4 py-3 text-sm text-rose-300"><AlertTriangle size={15}/>{actionError}</div>}

      {/* Table */}
      <div className="panel overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full min-w-[1020px] text-left text-sm">
            <thead className="bg-black/10 text-[11px] uppercase tracking-wider text-slate-500">
              <tr>
                <th className="px-4 py-3">Hostname</th>
                <th className="px-4 py-3">Address</th>
                <th className="px-4 py-3">Type</th>
                <th className="px-4 py-3">Location</th>
                <th className="px-4 py-3">Status</th>
                <th className="px-4 py-3">RTT</th>
                <th className="px-4 py-3">Interfaces</th>
                <th className="px-4 py-3">Alarms</th>
                <th className="px-4 py-3">Last poll</th>
                <th className="px-4 py-3 text-right">Manage</th>
              </tr>
            </thead>
            <tbody>
              {devices.map(device => (
                <tr key={device.id}
                  onClick={() => setSelectedId(device.id)}
                  className="cursor-pointer border-t border-white/[.05] transition hover:bg-white/[.025]">
                  <td className="px-4 py-3 font-medium text-slate-200">{device.hostname}</td>
                  <td className="px-4 py-3 font-mono text-xs text-slate-400">{device.ipAddress}</td>
                  <td className="px-4 py-3 text-slate-400">{device.deviceType}</td>
                  <td className="px-4 py-3 text-slate-400">{device.location}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-medium ${statusColors[device.status] ?? statusColors.unknown}`}>
                      <span className="size-1.5 rounded-full bg-current"/>
                      {device.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-400">{device.rttMs != null ? `${device.rttMs.toFixed(1)} ms` : '—'}</td>
                  <td className="px-4 py-3 text-xs">
                    <span className="text-emerald-400">{device.upInterfaces}↑</span>
                    {device.downInterfaces > 0 && <span className="ml-1 text-rose-400">{device.downInterfaces}↓</span>}
                  </td>
                  <td className="px-4 py-3">
                    {device.openAlarms > 0
                      ? <span className="inline-flex items-center gap-1 rounded bg-amber-400/10 px-2 py-0.5 text-xs text-amber-300"><AlertTriangle size={11}/>{device.openAlarms}</span>
                      : <span className="text-xs text-slate-600">—</span>}
                  </td>
                  <td className="px-4 py-3 text-xs text-slate-600">{device.lastPoll ? formatTime(device.lastPoll) : '—'}</td>
                  <td className="px-4 py-3"><div className="flex justify-end gap-1">
                    <button aria-label={`Edit ${device.hostname}`} title="Edit device" onClick={event=>{event.stopPropagation();setActionError('');setSelectedId(null);setEditorDevice(device)}} className="grid size-8 place-items-center rounded-lg border border-white/8 text-slate-500 hover:border-[#d6ff8a]/30 hover:text-[#d6ff8a]"><Pencil size={14}/></button>
                    <button aria-label={`Delete ${device.hostname}`} title="Delete device" onClick={event=>{event.stopPropagation();setActionError('');setDeleteTarget(device)}} className="grid size-8 place-items-center rounded-lg border border-white/8 text-slate-500 hover:border-rose-400/30 hover:text-rose-300"><Trash2 size={14}/></button>
                  </div></td>
                </tr>
              ))}
            </tbody>
          </table>
          {!loading && !devices.length && <div className="p-12 text-center text-sm text-slate-500">No devices match the current filters.</div>}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between border-t border-white/8 px-4 py-3">
            <button disabled={page <= 1} onClick={() => setPage(p => p-1)}
              className="flex items-center gap-1 rounded-lg border border-white/8 px-3 py-1.5 text-xs text-slate-400 disabled:opacity-40 hover:border-emerald-400/30 hover:text-emerald-300 transition">
              <ChevronLeft size={14}/> Previous
            </button>
            <span className="text-xs text-slate-500">Page {page} of {totalPages}</span>
            <button disabled={page >= totalPages} onClick={() => setPage(p => p+1)}
              className="flex items-center gap-1 rounded-lg border border-white/8 px-3 py-1.5 text-xs text-slate-400 disabled:opacity-40 hover:border-emerald-400/30 hover:text-emerald-300 transition">
              Next <ChevronRight size={14}/>
            </button>
          </div>
        )}
      </div>

      {selectedId != null && (
        <DeviceDetailPanel deviceId={selectedId} onClose={() => setSelectedId(null)}/>
      )}
      {editorDevice !== undefined&&<DeviceEditor device={editorDevice} models={models} onClose={()=>setEditorDevice(undefined)} onSave={saveDevice}/>}
      {deleteTarget&&<div className="fixed inset-0 z-[60] grid place-items-center bg-black/70 p-4 backdrop-blur-sm"><section role="alertdialog" aria-modal="true" className="w-full max-w-md rounded-2xl border border-white/10 bg-[#0c1210] p-6 shadow-2xl"><div className="flex items-start gap-3"><div className="grid size-10 shrink-0 place-items-center rounded-xl bg-rose-400/10 text-rose-300"><Trash2 size={18}/></div><div><h2 className="font-semibold text-white">Delete {deleteTarget.hostname}?</h2><p className="mt-2 text-sm leading-6 text-slate-400">This removes the device from inventory and deletes its stored topology links. This action cannot be undone.</p></div></div>{actionError&&<p role="alert" className="mt-4 text-sm text-rose-300">{actionError}</p>}<div className="mt-6 flex justify-end gap-2"><button onClick={()=>setDeleteTarget(null)} className="rounded-lg border border-white/10 px-4 py-2 text-sm text-slate-400 hover:text-white">Cancel</button><button disabled={saving} onClick={deleteDevice} className="inline-flex items-center gap-2 rounded-lg bg-rose-400 px-4 py-2 text-sm font-bold text-[#1b090c] hover:bg-rose-300 disabled:opacity-50">{saving&&<RefreshCw size={14} className="animate-spin"/>}Delete device</button></div></section></div>}
    </div>
  )
}

// ────────────────────────── HELPERS ──────────────────────────
function formatTime(value:string) { return value ? new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value)) : '—' }

// ────────────────────────── APP ROOT ──────────────────────────
export default function App() {
  const [authenticated, setAuthenticated] = useState<boolean|null>(null)
  const [data, setData] = useState(emptyDashboard)
  const [view, setView] = useState<View>('overview')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try { setData(await dashboardApi.dashboard()); setError('') }
    catch (err) { if (err instanceof ApiError && err.status===401) setAuthenticated(false); else setError('Telemetry service is temporarily unavailable.') }
    finally { setLoading(false) }
  }, [])

  useEffect(()=>{ dashboardApi.session().then(()=>setAuthenticated(true)).catch(()=>setAuthenticated(false)) },[])
  useEffect(()=>{ if(!authenticated)return; const first=window.setTimeout(load,0); const id=window.setInterval(load,15000); return()=>{ window.clearTimeout(first); window.clearInterval(id) } },[authenticated,load])

  if(authenticated===null) return <div className="grid min-h-screen place-items-center bg-[#07100f] text-emerald-300"><RefreshCw className="animate-spin"/></div>
  if(!authenticated) return <Login onAuthenticated={()=>setAuthenticated(true)}/>

  const nav: {id:View,label:string,icon:React.ReactNode,count?:number}[] = [
    {id:'overview',label:'Operations',icon:<LayoutDashboard size={18}/>},
    {id:'topology',label:'Network topology',icon:<Network size={18}/>,count:data.topology.length},
    {id:'inventory',label:'Fleet inventory',icon:<Server size={18}/>,count:data.overview.monitored},
    {id:'alerts',label:'Active alerts',icon:<AlertTriangle size={18}/>,count:data.overview.openAlerts},
    {id:'audit',label:'Audit events',icon:<ClipboardList size={18}/>},
  ]

  return (
    <div className="min-h-screen bg-[#07100f] text-slate-300">
      {/* Sidebar */}
      <aside className="fixed inset-y-0 left-0 z-20 hidden w-64 border-r border-white/7 bg-[#091312] lg:block">
        <div className="flex h-20 items-center gap-3 border-b border-white/7 px-5">
          <img src="/ccl-logo-dark.png" alt="Code Crafted Labs" className="size-10 rounded-full object-cover" />
          <div><p className="text-xs font-bold tracking-wide text-white">CODE CRAFTED LABS</p><p className="text-[10px] uppercase tracking-[.18em] text-[#d6ff8a]">NetPulse · Command center</p></div>
        </div>
        <nav className="p-4">
          <p className="px-3 py-3 text-[10px] font-bold uppercase tracking-[.2em] text-slate-600">Monitor</p>
          {nav.map(item=>(
            <button key={item.id} onClick={()=>setView(item.id)}
              className={`mb-1 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition ${view===item.id?'bg-emerald-400/10 text-emerald-300':'text-slate-500 hover:bg-white/[.035] hover:text-slate-300'}`}>
              {item.icon}<span className="flex-1 text-left">{item.label}</span>
              {item.count!=null&&<span className="rounded bg-black/20 px-1.5 py-0.5 text-[10px]">{item.count}</span>}
            </button>
          ))}
          {/* Alarm matrix badge */}
          <div className="mt-4 rounded-xl border border-amber-400/20 bg-amber-400/5 p-3">
            <div className="flex items-center gap-2 text-xs font-semibold text-amber-300"><Zap size={13}/>Alarm rules active</div>
            <p className="mt-1.5 text-[10px] leading-relaxed text-slate-500">6 strategies: device-down, link-state, admin/oper mismatch, high CPU, high memory, SNMP failure</p>
          </div>
        </nav>
        <div className="absolute bottom-[136px] w-full px-6 text-[10px] tracking-wide text-slate-600">A network operations platform by <span className="text-slate-400">Code Crafted Labs</span></div>
        <div className="absolute bottom-0 w-full border-t border-white/7 p-4">
          <div className="mb-3 flex items-center gap-3 rounded-lg bg-white/[.025] p-3">
            <div className="grid size-8 place-items-center rounded-full bg-emerald-400/10 text-emerald-300"><ShieldCheck size={16}/></div>
            <div><p className="text-xs font-medium text-slate-300">Administrator</p><p className="text-[10px] text-emerald-400">Secure session</p></div>
          </div>
          <button onClick={async()=>{await dashboardApi.logout();setAuthenticated(false)}} className="flex w-full items-center gap-2 px-3 py-2 text-xs text-slate-500 hover:text-rose-300"><LogOut size={15}/>Terminate session</button>
        </div>
      </aside>

      {/* Main content */}
      <div className="lg:pl-64">
        <header className="sticky top-0 z-10 flex h-20 items-center gap-4 border-b border-white/7 bg-[#07100f]/90 px-5 backdrop-blur-xl sm:px-8">
          <div className="lg:hidden flex items-center gap-2"><img src="/ccl-logo-dark.png" alt="Code Crafted Labs" className="size-8 rounded-full object-cover"/><Radio className="text-[#d6ff8a]"/></div>
          <div className="flex-1"/>
          <button onClick={load} aria-label="Refresh dashboard" className="grid size-9 place-items-center rounded-lg border border-white/8 text-slate-500 hover:text-emerald-300">
            <RefreshCw className={loading?'animate-spin':''} size={16}/>
          </button>
          <div className="hidden items-center gap-2 text-xs text-slate-500 sm:flex">
            <span className="size-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399]"/>Pipeline connected
          </div>
        </header>

        <main className="mx-auto max-w-[1500px] p-5 sm:p-8">
          <div className="mb-7 flex flex-wrap items-end justify-between gap-3">
            <div>
              <p className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[.18em] text-emerald-400"><Activity size={13}/>Live telemetry</p>
              <h1 className="text-2xl font-semibold tracking-tight text-white">{nav.find(item=>item.id===view)?.label}</h1>
            </div>
            <p className="text-xs text-slate-600">Last synchronized {formatTime(data.generatedAt)}</p>
          </div>
          {error&&<div role="alert" className="mb-4 flex items-center gap-2 rounded-lg border border-amber-400/20 bg-amber-400/8 p-3 text-sm text-amber-200"><AlertTriangle size={16}/>{error}</div>}
          {view==='overview'&&<Overview data={data}/>}
          {view==='topology'&&<TopologyMap data={data}/>}
          {view==='inventory'&&<InventoryView/>}
          {view==='alerts'&&<Alerts events={data.events}/>}
          {view==='audit'&&<AuditView/>}
        </main>
      </div>
    </div>
  )
}
