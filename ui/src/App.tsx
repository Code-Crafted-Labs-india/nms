import { useCallback, useEffect, useMemo, useState } from 'react'
import type { FormEvent } from 'react'
import {
  Activity, AlertTriangle, Bell, Boxes, CircleCheck, Cpu, LayoutDashboard,
  LockKeyhole, LogOut, MemoryStick, Network, Radio, RefreshCw, Search,
  Server, ShieldCheck, Wifi, WifiOff,
} from 'lucide-react'
import { ApiError, dashboardApi, type DashboardData, type MetricPoint } from './api'

type View = 'overview' | 'topology' | 'devices' | 'alerts'
const emptyDashboard: DashboardData = { generatedAt: '', overview: { monitored: 0, online: 0, offline: 0, openAlerts: 0 }, devices: [], events: [], latency: [], cpu: [], memory: [], topology: [] }

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
      <div className="mb-8 flex items-center gap-3"><div className="grid size-11 place-items-center rounded-xl border border-emerald-400/30 bg-emerald-400/10 text-emerald-300"><ShieldCheck size={23} /></div><div><p className="text-xs font-semibold uppercase tracking-[.24em] text-emerald-400">NetPulse Core</p><h1 className="text-xl font-semibold">Secure operations console</h1></div></div>
      <div className="mb-6 rounded-lg border border-white/8 bg-white/[.025] p-4 text-sm leading-6 text-slate-400"><div className="mb-1 flex items-center gap-2 font-medium text-slate-200"><LockKeyhole size={15} /> Restricted system</div>Authorized personnel only. Activity is subject to monitoring and audit.</div>
      <form onSubmit={submit}><label className="mb-2 block text-xs font-semibold uppercase tracking-wider text-slate-400" htmlFor="access-token">Operator credential</label><input id="access-token" type="password" autoComplete="current-password" required minLength={32} value={token} onChange={e => setToken(e.target.value)} className="w-full rounded-lg border border-white/10 bg-black/20 px-4 py-3 text-sm outline-none transition focus:border-emerald-400/60 focus:ring-2 focus:ring-emerald-400/10" />{error && <p role="alert" className="mt-3 text-sm text-rose-300">{error}</p>}<button disabled={busy} className="mt-5 flex w-full items-center justify-center gap-2 rounded-lg bg-emerald-400 px-4 py-3 text-sm font-bold text-[#06100e] transition hover:bg-emerald-300 disabled:opacity-50">{busy ? <RefreshCw className="animate-spin" size={16} /> : <LockKeyhole size={16} />} Authenticate</button></form>
      <p className="mt-6 text-center text-xs text-slate-600">Credentials are exchanged for an HttpOnly session and are never stored by this client.</p>
    </section>
  </main>
}

function Sparkline({ points, color = '#34d399' }: { points: MetricPoint[], color?: string }) {
  const values = points.slice(-36).map(point => point.value)
  if (values.length < 2) return <div className="grid h-28 place-items-center text-xs text-slate-600">Awaiting telemetry</div>
  const min = Math.min(...values), max = Math.max(...values), range = Math.max(max - min, 1)
  const line = values.map((value, index) => `${(index / (values.length - 1)) * 300},${100 - ((value - min) / range) * 80}`).join(' ')
  return <svg viewBox="0 0 300 110" className="h-28 w-full overflow-visible" role="img" aria-label="Recent metric trend">{[20, 60, 100].map(y => <line key={y} x1="0" x2="300" y1={y} y2={y} stroke="rgba(148,163,184,.09)" />)}<polyline points={line} fill="none" stroke={color} strokeWidth="2.2" strokeLinejoin="round" strokeLinecap="round" /></svg>
}

function Stat({ label, value, detail, icon, tone = 'emerald' }: { label: string, value: number, detail: string, icon: React.ReactNode, tone?: 'emerald'|'rose'|'amber'|'sky' }) {
  const tones = { emerald:'text-emerald-300 bg-emerald-400/10 border-emerald-400/20', rose:'text-rose-300 bg-rose-400/10 border-rose-400/20', amber:'text-amber-300 bg-amber-400/10 border-amber-400/20', sky:'text-sky-300 bg-sky-400/10 border-sky-400/20' }
  return <article className="panel p-5"><div className="flex items-start justify-between"><div><p className="label">{label}</p><p className="mt-3 text-3xl font-semibold tracking-tight text-white">{value}</p></div><div className={`grid size-10 place-items-center rounded-lg border ${tones[tone]}`}>{icon}</div></div><p className="mt-4 text-xs text-slate-500">{detail}</p></article>
}

function MetricCard({ title, icon, points, color, suffix }: { title:string, icon:React.ReactNode, points:MetricPoint[], color:string, suffix:string }) {
  const last = points.at(-1)?.value
  return <article className="panel p-5"><header className="flex items-center justify-between"><div className="flex items-center gap-2 text-sm font-semibold text-slate-200">{icon}{title}</div><div className="text-lg font-semibold text-white">{last == null ? '—' : `${last.toFixed(1)}${suffix}`}</div></header><div className="mt-2"><Sparkline points={points} color={color}/></div></article>
}

function DeviceTable({ devices }: { devices: DashboardData['devices'] }) {
  return <section className="panel mt-4 overflow-hidden"><header className="flex items-center justify-between border-b border-white/7 px-5 py-4"><div><p className="label">Asset status</p><h2 className="mt-1 font-semibold text-white">Managed infrastructure</h2></div><Network className="text-slate-500" size={20}/></header><div className="overflow-x-auto"><table className="w-full min-w-[700px] text-left text-sm"><thead className="bg-black/10 text-[11px] uppercase tracking-wider text-slate-500"><tr><th>Asset</th><th>Address</th><th>Role</th><th>Location</th><th>Latency</th><th>Status</th></tr></thead><tbody>{devices.map(device=><tr key={device.id} className="border-t border-white/[.055]"><td><div className="font-medium text-slate-200">{device.hostname}</div></td><td className="font-mono text-xs text-slate-400">{device.ipAddress}</td><td>{device.type}</td><td>{device.location}</td><td>{device.rttMs == null ? '—' : `${device.rttMs.toFixed(1)} ms`}</td><td><span className={`status ${device.status === 'online' ? 'status-up':'status-down'}`}><span className="size-1.5 rounded-full bg-current"/>{device.status}</span></td></tr>)}</tbody></table>{!devices.length && <div className="p-10 text-center text-sm text-slate-500">No managed assets returned by the telemetry service.</div>}</div></section>
}

function TopologyMap({ data }: { data: DashboardData }) {
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
            return <g key={node.name} transform={`translate(${node.x} ${node.y})`}><circle r="38" fill={color} opacity=".08" filter="url(#nodeGlow)"/><rect x="-72" y="-31" width="144" height="62" rx="12" fill="#0d1917" stroke={color} strokeWidth="2"/><circle cx="-50" cy="-8" r="7" fill={color}/><text x="-36" y="-3" fill="#e2e8f0" fontSize="13" fontWeight="600">{node.name.length > 17 ? `${node.name.slice(0,16)}…` : node.name}</text><text x="-50" y="17" fill="#64748b" fontSize="10">{node.device?.type || 'Unmanaged device'}</text><title>{node.name} — {node.device?.status || 'unknown'}{node.device?.ipAddress ? ` — ${node.device.ipAddress}` : ''}</title></g>
          })}
        </svg>
        {!data.topology.length && <p className="pb-5 text-center text-xs text-amber-300">No LLDP links discovered yet. Devices are displayed without connections.</p>}
      </div>
    </article>
    <aside className="space-y-4"><article className="panel p-5"><p className="label">Topology summary</p><div className="mt-4 space-y-3"><div className="flex justify-between text-sm text-slate-400"><span>Visible nodes</span><strong className="text-white">{nodes.length}</strong></div><div className="h-px bg-white/7"/><div className="flex justify-between text-sm text-slate-400"><span>Discovered links</span><strong className="text-white">{data.topology.length}</strong></div><div className="h-px bg-white/7"/><div className="flex justify-between text-sm text-slate-400"><span>Down devices</span><strong className={offline.length ? 'text-rose-300':'text-emerald-300'}>{offline.length}</strong></div></div></article><article className="panel overflow-hidden"><header className="border-b border-white/7 p-4"><p className="label">Service impact</p></header>{offline.length ? <div className="divide-y divide-white/[.06]">{offline.map(device=><div key={device.id} className="flex items-center gap-3 p-4"><span className="size-2 rounded-full bg-rose-400 shadow-[0_0_8px_#fb7185]"/><div><p className="text-sm font-medium text-slate-200">{device.hostname}</p><p className="text-xs text-slate-500">{device.ipAddress}</p></div></div>)}</div> : <div className="grid place-items-center gap-2 p-8 text-center text-sm text-slate-500"><CircleCheck className="text-emerald-400" size={24}/>All visible devices operational</div>}</article></aside>
  </section>
}

function Overview({ data }: { data: DashboardData }) {
  const latency = useMemo(() => data.latency.slice(-80), [data.latency])
  const health = data.overview.monitored ? Math.round(data.overview.online / data.overview.monitored * 100) : 0
  return <><section className="grid gap-4 sm:grid-cols-2 xl:grid-cols-4"><Stat label="Monitored assets" value={data.overview.monitored} detail="Inventory under active supervision" icon={<Server size={19}/>} tone="sky" /><Stat label="Operational" value={data.overview.online} detail={`${health}% fleet availability`} icon={<Wifi size={19}/>} /><Stat label="Unreachable" value={data.overview.offline} detail="No response within five minutes" icon={<WifiOff size={19}/>} tone="rose" /><Stat label="Open alerts" value={data.overview.openAlerts} detail="Unresolved operational events" icon={<Bell size={19}/>} tone="amber" /></section>
    <section className="mt-4 grid gap-4 xl:grid-cols-[1.6fr_1fr]"><article className="panel p-5"><header className="flex items-center justify-between"><div><p className="label">Network latency</p><h2 className="mt-1 font-semibold text-white">Response time · last 30 minutes</h2></div><div className="flex items-center gap-2 text-xs text-emerald-300"><span className="size-2 rounded-full bg-emerald-400"/> Live</div></header><div className="mt-5"><Sparkline points={latency}/></div></article><article className="panel p-5"><p className="label">Fleet posture</p><div className="mt-5 flex items-center gap-6"><div className="relative grid size-28 shrink-0 place-items-center rounded-full" style={{background:`conic-gradient(#34d399 ${health}%, #25302e 0)`}}><div className="grid size-20 place-items-center rounded-full bg-[#0d1817]"><span className="text-2xl font-semibold text-white">{health}%</span></div></div><div className="w-full space-y-3 text-sm"><div className="flex justify-between text-slate-400"><span>Operational</span><strong className="text-emerald-300">{data.overview.online}</strong></div><div className="h-px bg-white/8"/><div className="flex justify-between text-slate-400"><span>Unreachable</span><strong className="text-rose-300">{data.overview.offline}</strong></div><div className="h-px bg-white/8"/><div className="flex justify-between text-slate-400"><span>Unclassified</span><strong className="text-slate-200">{Math.max(data.overview.monitored-data.overview.online-data.overview.offline,0)}</strong></div></div></div></article></section>
    <section className="mt-4 grid gap-4 xl:grid-cols-2"><MetricCard title="CPU utilization" icon={<Cpu size={17}/>} points={data.cpu} color="#38bdf8" suffix="%" /><MetricCard title="Memory utilization" icon={<MemoryStick size={17}/>} points={data.memory} color="#a78bfa" suffix="%" /></section><DeviceTable devices={data.devices.slice(0, 6)} /></>
}

function Alerts({ events }: { events: DashboardData['events'] }) {
  return <section className="panel overflow-hidden"><header className="border-b border-white/7 px-5 py-5"><p className="label">Event stream</p><h2 className="mt-1 text-lg font-semibold text-white">Active alerts</h2></header><div className="divide-y divide-white/[.06]">{events.map((event,index)=><article key={`${event.time}-${index}`} className="flex gap-4 p-5"><div className={`mt-0.5 grid size-9 shrink-0 place-items-center rounded-lg ${event.severity==='CRITICAL'?'bg-rose-400/10 text-rose-300':'bg-amber-400/10 text-amber-300'}`}><AlertTriangle size={17}/></div><div className="min-w-0 flex-1"><div className="flex flex-wrap items-center gap-2"><strong className="text-sm text-slate-200">{event.type}</strong><span className="text-xs text-slate-500">{event.device}</span></div><p className="mt-1 text-sm text-slate-400">{event.message}</p><time className="mt-2 block text-xs text-slate-600">{formatTime(event.time)}</time></div></article>)}{!events.length&&<div className="grid place-items-center gap-3 p-16 text-sm text-slate-500"><CircleCheck size={28} className="text-emerald-400"/>No unresolved alerts</div>}</div></section>
}

function formatTime(value:string) { return value ? new Intl.DateTimeFormat(undefined,{dateStyle:'medium',timeStyle:'short'}).format(new Date(value)) : '—' }

export default function App() {
  const [authenticated, setAuthenticated] = useState<boolean|null>(null), [data, setData] = useState(emptyDashboard), [view, setView] = useState<View>('overview'), [loading, setLoading] = useState(false), [error, setError] = useState(''), [query, setQuery] = useState('')
  const load = useCallback(async () => { setLoading(true); try { setData(await dashboardApi.dashboard()); setError('') } catch (err) { if (err instanceof ApiError && err.status===401) setAuthenticated(false); else setError('Telemetry service is temporarily unavailable.') } finally { setLoading(false) } }, [])
  useEffect(()=>{ dashboardApi.session().then(()=>setAuthenticated(true)).catch(()=>setAuthenticated(false)) },[])
  useEffect(()=>{ if(!authenticated)return; const first=window.setTimeout(load,0); const id=window.setInterval(load,15000); return()=>{ window.clearTimeout(first); window.clearInterval(id) } },[authenticated,load])
  if(authenticated===null) return <div className="grid min-h-screen place-items-center bg-[#07100f] text-emerald-300"><RefreshCw className="animate-spin"/></div>
  if(!authenticated) return <Login onAuthenticated={()=>setAuthenticated(true)}/>
  const devices = data.devices.filter(device => `${device.hostname} ${device.ipAddress} ${device.type}`.toLowerCase().includes(query.toLowerCase()))
  const nav: {id:View,label:string,icon:React.ReactNode,count?:number}[] = [{id:'overview',label:'Operations',icon:<LayoutDashboard size={18}/>},{id:'topology',label:'Network topology',icon:<Network size={18}/>,count:data.topology.length},{id:'devices',label:'Infrastructure',icon:<Boxes size={18} />,count:data.overview.monitored},{id:'alerts',label:'Active alerts',icon:<AlertTriangle size={18}/>,count:data.overview.openAlerts}]
  return <div className="min-h-screen bg-[#07100f] text-slate-300"><aside className="fixed inset-y-0 left-0 z-20 hidden w-64 border-r border-white/7 bg-[#091312] lg:block"><div className="flex h-20 items-center gap-3 border-b border-white/7 px-6"><div className="grid size-9 place-items-center rounded-lg bg-emerald-400 text-[#07100f]"><Radio size={20}/></div><div><p className="text-sm font-bold tracking-wide text-white">NETPULSE</p><p className="text-[10px] uppercase tracking-[.2em] text-emerald-400">Command center</p></div></div><nav className="p-4"><p className="px-3 py-3 text-[10px] font-bold uppercase tracking-[.2em] text-slate-600">Monitor</p>{nav.map(item=><button key={item.id} onClick={()=>setView(item.id)} className={`mb-1 flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-sm transition ${view===item.id?'bg-emerald-400/10 text-emerald-300':'text-slate-500 hover:bg-white/[.035] hover:text-slate-300'}`}>{item.icon}<span className="flex-1 text-left">{item.label}</span>{item.count!=null&&<span className="rounded bg-black/20 px-1.5 py-0.5 text-[10px]">{item.count}</span>}</button>)}</nav><div className="absolute bottom-0 w-full border-t border-white/7 p-4"><div className="mb-3 flex items-center gap-3 rounded-lg bg-white/[.025] p-3"><div className="grid size-8 place-items-center rounded-full bg-emerald-400/10 text-emerald-300"><ShieldCheck size={16}/></div><div><p className="text-xs font-medium text-slate-300">Administrator</p><p className="text-[10px] text-emerald-400">Secure session</p></div></div><button onClick={async()=>{await dashboardApi.logout();setAuthenticated(false)}} className="flex w-full items-center gap-2 px-3 py-2 text-xs text-slate-500 hover:text-rose-300"><LogOut size={15}/>Terminate session</button></div></aside><div className="lg:pl-64"><header className="sticky top-0 z-10 flex h-20 items-center gap-4 border-b border-white/7 bg-[#07100f]/90 px-5 backdrop-blur-xl sm:px-8"><div className="lg:hidden"><Radio className="text-emerald-400"/></div><div className="relative max-w-md flex-1"><Search className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-600" size={16}/><input value={query} onChange={e=>setQuery(e.target.value)} placeholder="Search assets or addresses" className="w-full rounded-lg border border-white/8 bg-white/[.025] py-2.5 pl-10 pr-3 text-sm outline-none focus:border-emerald-400/40"/></div><button onClick={load} aria-label="Refresh dashboard" className="grid size-9 place-items-center rounded-lg border border-white/8 text-slate-500 hover:text-emerald-300"><RefreshCw className={loading?'animate-spin':''} size={16}/></button><div className="hidden items-center gap-2 text-xs text-slate-500 sm:flex"><span className="size-2 rounded-full bg-emerald-400 shadow-[0_0_8px_#34d399]"/>Pipeline connected</div></header><main className="mx-auto max-w-[1500px] p-5 sm:p-8"><div className="mb-7 flex flex-wrap items-end justify-between gap-3"><div><p className="mb-2 flex items-center gap-2 text-xs font-semibold uppercase tracking-[.18em] text-emerald-400"><Activity size={13}/>Live telemetry</p><h1 className="text-2xl font-semibold tracking-tight text-white">{nav.find(item=>item.id===view)?.label}</h1></div><p className="text-xs text-slate-600">Last synchronized {formatTime(data.generatedAt)}</p></div>{error&&<div role="alert" className="mb-4 flex items-center gap-2 rounded-lg border border-amber-400/20 bg-amber-400/8 p-3 text-sm text-amber-200"><AlertTriangle size={16}/>{error}</div>}{view==='overview'&&<Overview data={data}/>} {view==='topology'&&<TopologyMap data={data}/>} {view==='devices'&&<DeviceTable devices={devices}/>} {view==='alerts'&&<Alerts events={data.events}/>}</main></div></div>
}
