import { useCallback,useEffect,useState,type FormEvent } from 'react'
import { useSession,type User } from './session'
import { type Department,type DepartmentOptions } from './Onboarding'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Badge } from '@/components/ui/badge'
const statusNames:Record<string,string>={PENDING:'승인 대기',ACTIVE:'이용 중',SUSPENDED:'이용 정지',REJECTED:'가입 반려'}
type Member=User&{email:string;createdAt:string}
function message(e:unknown){return e instanceof Error?e.message:'연결을 확인하고 다시 시도해 주세요.'}
export function MemberAdmin(){
 const {api,state}=useSession();const [members,setMembers]=useState<Member[]|null>(null);const [error,setError]=useState('');const [busy,setBusy]=useState(false)
 const load=useCallback(async()=>{setError('');try{const data=await api<{members:Member[]}>('/admin/members');setMembers(data.members)}catch(e){setError(message(e))}},[api])
 useEffect(()=>{void load()},[load])
 async function review(member:Member,action:string){if(!window.confirm(`${member.realName} 님을 ${action==='approve'?'가입 승인':action==='reject'?'가입 반려':action==='suspend'?'이용 정지':'정지 해제'}하시겠습니까? 인사팀 소속을 승인하면 관리자 권한이 부여됩니다.`))return;setBusy(true);setError('');try{await api(`/admin/members/${encodeURIComponent(member.id)}/${action}`,{method:'POST',body:{}});await load()}catch(e){setError(message(e))}finally{setBusy(false)}}
 return <section className="py-8"><div className="flex flex-wrap items-center justify-between gap-4"><h1 className="text-2xl font-bold">구성원 관리</h1><Button type="button" variant="outline" disabled={busy} onClick={()=>void load()}>새로고침</Button></div><p className="mt-3 text-muted-foreground">이름·메일·소속을 확인하고 가입을 승인해 주세요. 인사팀 소속을 승인하면 관리자 권한을 받습니다.</p>
 {error?<p role="alert" className="mt-4 text-destructive">{error}</p>:null}
 {members===null?<p role="status" className="py-8">구성원을 불러오고 있습니다.</p>:members.length===0?<p className="py-8">아직 가입 신청한 구성원이 없습니다.</p>:<ul className="mt-6 divide-y border-y">{members.map(m=><li key={m.id} className="flex flex-wrap items-center justify-between gap-4 py-5"><div className="min-w-0"><div className="flex flex-wrap items-center gap-2"><h2 className="font-semibold">{m.realName}</h2><Badge variant={m.status==='PENDING'?'secondary':'outline'}>{statusNames[m.status]??m.status}</Badge>{m.role==='HR_ADMIN'?<Badge>관리자</Badge>:null}</div><p className="mt-2 break-all text-sm">{m.email}</p><p className="mt-1 text-sm text-muted-foreground">{m.department} · {m.jobLevel}</p></div><div className="flex flex-wrap gap-2">{m.status==='PENDING'?<><Button type="button" disabled={busy} onClick={()=>void review(m,'approve')}>가입 승인</Button><Button type="button" variant="outline" disabled={busy} onClick={()=>void review(m,'reject')}>반려</Button></>:m.id!==(state.status==='authenticated'?state.user.id:'')&&m.status!=='REJECTED'?<Button type="button" variant="outline" disabled={busy} onClick={()=>void review(m,m.status==='SUSPENDED'?'restore':'suspend')}>{m.status==='SUSPENDED'?'정지 해제':'이용 정지'}</Button>:null}</div></li>)}</ul>}</section>
}
export function DepartmentAdmin(){
 const {api}=useSession();const [items,setItems]=useState<Department[]|null>(null);const [name,setName]=useState('');const [error,setError]=useState('');const [busy,setBusy]=useState(false)
 const load=useCallback(async()=>{try{const data=await api<DepartmentOptions>('/admin/departments');setItems(data.departments)}catch(e){setError(message(e))}},[api]);useEffect(()=>{void load()},[load])
 async function change(method:string,path:string,body?:unknown){setBusy(true);setError('');try{await api(path,{method,body});if(method==='POST')setName('');await load();return true}catch(e){setError(message(e));return false}finally{setBusy(false)}}
 async function create(e:FormEvent){e.preventDefault();await change('POST','/admin/departments',{name})}
 return <section className="mx-auto max-w-2xl py-8"><h1 className="text-2xl font-bold">부서 관리</h1><p className="mt-3 text-muted-foreground">가입 신청에서 선택할 부서를 관리합니다. 인사팀의 관리자 지정은 이름을 바꿔도 유지됩니다.</p>
 <form className="mt-6 space-y-2" onSubmit={create}><Label htmlFor="new-department">새 부서 이름</Label><div className="flex gap-2"><Input id="new-department" required maxLength={50} value={name} onChange={e=>setName(e.target.value)}/><Button type="submit" disabled={busy}>추가</Button></div></form>
 {error?<p role="alert" className="mt-4 text-destructive">{error}</p>:null}
 {items===null?<p role="status" className="py-8">부서를 불러오고 있습니다.</p>:<ul className="mt-6 divide-y border-y">{items.map(d=><DepartmentRow key={d.id} department={d} busy={busy} change={change}/>)}</ul>}
 <Button type="button" variant="ghost" className="mt-4" disabled={busy} onClick={()=>void load()}>목록 새로고침</Button><p className="mt-4 text-sm text-muted-foreground">인사팀과 구성원이 선택한 부서는 삭제할 수 없습니다.</p></section>
}
function DepartmentRow({department:d,busy,change}:{department:Department;busy:boolean;change:(method:string,path:string,body?:unknown)=>Promise<boolean>}){
 const [editing,setEditing]=useState(false);const [name,setName]=useState(d.name);const path=`/admin/departments/${encodeURIComponent(d.id)}`
 async function save(e:FormEvent){e.preventDefault();if(await change('PATCH',path,{name}))setEditing(false)}
 return <li className="flex flex-wrap items-center justify-between gap-3 py-4">{editing?<form onSubmit={save} className="flex w-full flex-wrap items-end gap-2"><div className="min-w-0 flex-1 space-y-2"><Label htmlFor={`department-${d.id}`}>부서 이름</Label><Input id={`department-${d.id}`} required maxLength={50} value={name} onChange={e=>setName(e.target.value)}/></div><Button type="submit" disabled={busy}>저장</Button><Button type="button" variant="ghost" disabled={busy} onClick={()=>setEditing(false)}>취소</Button></form>:<><span className="break-all font-medium">{d.name} {d.isHR?<Badge variant="secondary">관리자 부서</Badge>:null}</span><div className="flex gap-2"><Button type="button" variant="outline" disabled={busy} onClick={()=>{setName(d.name);setEditing(true)}} aria-label={`${d.name} 이름 수정`}>이름 수정</Button>{!d.isHR?<Button type="button" variant="ghost" disabled={busy} onClick={()=>{if(window.confirm(`${d.name} 부서를 삭제하시겠습니까?`))void change('DELETE',path)}} aria-label={`${d.name} 삭제`}>삭제</Button>:null}</div></>}</li>
}
