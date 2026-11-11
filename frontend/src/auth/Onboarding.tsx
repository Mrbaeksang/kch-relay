import { useEffect,useState,type FormEvent } from 'react'
import { useSession } from './session'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
export type Department={id:string;name:string;isHR:boolean}
export type DepartmentOptions={departments:Department[];jobLevels:string[]}
const fieldClass='min-h-11 w-full rounded-md border border-input bg-background px-3 text-base'
export function Onboarding(){
 const {state,api,refresh,logout}=useSession()
 const [options,setOptions]=useState<DepartmentOptions|null>(null)
 const [error,setError]=useState('');const [busy,setBusy]=useState(false)
 const [realName,setRealName]=useState(state.status==='authenticated'?state.user.realName:'');const [departmentId,setDepartmentId]=useState('');const [jobLevel,setJobLevel]=useState(state.status==='authenticated'?state.user.jobLevel:'')
 const status=state.status==='authenticated'?state.user.status:''
 useEffect(()=>{if(status==='PROFILE_REQUIRED'||status==='REJECTED'){const c=new AbortController();api<DepartmentOptions>('/auth/departments',{signal:c.signal}).then(data=>{setOptions(data);if(state.status==='authenticated')setDepartmentId(data.departments.find(d=>d.name===state.user.department)?.id??'')}).catch(e=>{if(!c.signal.aborted)setError(e instanceof Error?e.message:'부서 목록을 불러오지 못했습니다.')});return()=>c.abort()}},[api,status])
 async function submit(e:FormEvent){e.preventDefault();setBusy(true);setError('');try{await api('/auth/register',{method:'POST',body:{realName,departmentId,jobLevel}});await refresh()}catch(e){setError(e instanceof Error?e.message:'가입 신청을 처리하지 못했습니다.')}finally{setBusy(false)}}
 async function check(){setBusy(true);setError('');try{await refresh()}catch(e){setError(e instanceof Error?e.message:'상태를 확인하지 못했습니다.')}finally{setBusy(false)}}
 return <section className="mx-auto max-w-lg py-10"><h1 className="text-2xl font-bold">{status==='PROFILE_REQUIRED'?'구성원 가입 신청':status==='PENDING'?'가입 승인을 기다리고 있습니다':'가입 신청이 반려되었습니다'}</h1>
 <p className="mt-3 text-muted-foreground">{(status==='PROFILE_REQUIRED'||status==='REJECTED')?status==='REJECTED'?'정보를 확인·수정한 뒤 다시 신청해 주세요. 재신청 후에도 인사팀 승인이 필요합니다.':'실제 이름과 소속을 입력해 주세요. 인사팀 승인 후 장터를 이용할 수 있습니다.':status==='PENDING'?'인사팀에서 신청 정보를 확인하고 있습니다. 승인 전에는 장터를 이용할 수 없습니다.':'소속 인사팀에 문의해 주세요.'}</p>
 {error?<p role="alert" className="mt-4 text-destructive">{error}</p>:null}
 {(status==='PROFILE_REQUIRED'||status==='REJECTED')?options?<form onSubmit={submit} className="mt-6 space-y-5"><div className="space-y-2"><Label htmlFor="real-name">이름</Label><Input id="real-name" autoComplete="name" required maxLength={50} value={realName} onChange={e=>setRealName(e.target.value)}/></div><div className="space-y-2"><Label htmlFor="department">부서</Label><select id="department" className={fieldClass} required value={departmentId} onChange={e=>setDepartmentId(e.target.value)}><option value="">부서를 선택해 주세요</option>{options.departments.map(d=><option key={d.id} value={d.id}>{d.name}</option>)}</select></div><div className="space-y-2"><Label htmlFor="job-level">직급</Label><select id="job-level" className={fieldClass} required value={jobLevel} onChange={e=>setJobLevel(e.target.value)}><option value="">직급을 선택해 주세요</option>{options.jobLevels.map(v=><option key={v}>{v}</option>)}</select></div><Button type="submit" disabled={busy}>{busy?'신청 중…':status==='REJECTED'?'가입 재신청':'가입 신청'}</Button></form>:<div className="mt-5"><p role="status">부서 목록을 불러오고 있습니다.</p>{error?<Button type="button" variant="outline" onClick={()=>window.location.reload()}>다시 불러오기</Button>:null}</div>:status==='PENDING'?<Button type="button" className="mt-6" disabled={busy} onClick={()=>void check()}>{busy?'확인 중…':'승인 상태 확인'}</Button>:null}
 {status==='REJECTED'?<Button type="button" variant="outline" className="mt-6" disabled={busy} onClick={()=>void check()}>승인 상태 확인</Button>:null}
 <Button type="button" variant="ghost" className="mt-6" disabled={busy} onClick={()=>void logout()}>로그아웃</Button></section>
}
export {fieldClass}
