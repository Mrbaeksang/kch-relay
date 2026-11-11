import { useEffect, useState } from 'react'
import { useSession } from '../auth/session'
import { prepareImage, type Limits } from '../products/types'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

export function PhotoEditor({ ids, onChange, busy, onBusy }: {ids:string[];onChange:(ids:string[])=>void;busy:boolean;onBusy:(busy:boolean)=>void}) {
 const {api}=useSession()
 const [limits,setLimits]=useState<Limits|null>(null)
 const [error,setError]=useState('')
 useEffect(()=>{const c=new AbortController();void api<Limits>('/product-limits',{signal:c.signal}).then(setLimits).catch(()=>{if(!c.signal.aborted)setError('사진 기준을 불러오지 못했습니다. 새로고침해 주세요.')});return()=>c.abort()},[api])
 async function add(files:FileList|null){
  if(!files?.length||!limits)return
  if(ids.length+files.length>limits.maxImagesPerProduct){setError(`사진은 최대 ${limits.maxImagesPerProduct}장입니다.`);return}
  onBusy(true);setError('');const next=[...ids]
  try{for(const file of Array.from(files)){
   if(!limits.imageTypes.includes(file.type))throw new Error('JPEG 또는 PNG 사진만 올릴 수 있습니다.')
   const photo=await prepareImage(file)
   if(photo.size>limits.maxImageBytes)throw new Error('사진 용량이 너무 큽니다.')
   const body=new FormData();body.append('file',photo,'photo')
   const uploaded=await api<{id:string}>('/images',{method:'POST',body});next.push(uploaded.id);onChange([...next])
  }}catch(e){setError(e instanceof Error?e.message:'사진을 올리지 못했습니다.')}finally{onBusy(false)}
 }
 return <fieldset className="my-4 space-y-3"><legend>상품 사진</legend>
 {ids.length ? <div className="flex flex-wrap gap-3">{ids.map((id,i)=><div key={id}><img className="size-24 rounded-lg object-cover" src={`/api/v1/images/${encodeURIComponent(id)}`} alt={`상품 사진 ${i+1}`} /><Button type="button" variant="outline" disabled={busy} onClick={()=>onChange(ids.filter(x=>x!==id))}>사진 {i+1} 빼기</Button></div>)}</div>:<p className="text-sm text-muted-foreground">등록된 사진이 없습니다.</p>}
 <label>사진 추가<Input type="file" accept={limits?.imageTypes.join(',')} multiple disabled={busy||!limits} onChange={e=>{void add(e.target.files);e.target.value=''}} /></label>
 <p className="text-sm text-muted-foreground">사진 변경도 내용 저장 때 반영되며, 대기 신청이 있으면 거절 동의가 필요합니다.</p>
 {error?<p role="alert">{error}</p>:null}</fieldset>
}
