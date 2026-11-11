import { cleanup,fireEvent,render,screen } from '@testing-library/react'
import { afterEach,expect,test,vi } from 'vitest'
import { App } from '../app/App'
afterEach(()=>{cleanup();vi.unstubAllGlobals()})
test('구글 로그인한 미가입자가 프로필을 제출하면 승인 대기로 전환하고 장터를 감춘다',async()=>{
 let pending=false
 vi.stubGlobal('fetch',vi.fn(async(input:RequestInfo|URL,init?:RequestInit)=>{
  const path=String(input)
  if(path.endsWith('/auth/config'))return Response.json({data:{authMode:'google',googleLogin:true,developmentLogin:false,accounts:[]}})
  if(path.endsWith('/auth/me'))return Response.json({data:{user:{id:'new',realName:'',department:'',jobLevel:'',role:'MEMBER',status:pending?'PENDING':'PROFILE_REQUIRED'},csrfToken:'token'}})
  if(path.endsWith('/auth/departments'))return Response.json({data:{departments:[{id:'hr',name:'인사팀',isHR:true}],jobLevels:['사원']}})
  if(path.endsWith('/auth/register')){expect(JSON.parse(String(init?.body))).toEqual({realName:'가상 직원',departmentId:'hr',jobLevel:'사원'});pending=true;return Response.json({data:{status:'PENDING'}})}
  throw Error('Unexpected protected API '+path)
 }))
 render(<App/>);fireEvent.change(await screen.findByLabelText('이름'),{target:{value:'가상 직원'}})
 fireEvent.change(screen.getByLabelText('부서'),{target:{value:'hr'}});fireEvent.change(screen.getByLabelText('직급'),{target:{value:'사원'}})
 fireEvent.click(screen.getByRole('button',{name:'가입 신청'}));await screen.findByRole('heading',{name:'가입 승인을 기다리고 있습니다'})
 expect(screen.queryByRole('link',{name:'내 상품'})).toBeNull()
})
test('구글 로그인 사용 환경에서 구글 로그인 링크를 제공한다',async()=>{
 vi.stubGlobal('fetch',async(input:RequestInfo|URL)=>String(input).endsWith('/auth/config')?Response.json({data:{authMode:'google',googleLogin:true,developmentLogin:false,accounts:[]}}):Response.json({error:{code:'UNAUTHENTICATED'}},{status:401}))
 render(<App/>);expect((await screen.findByRole('link',{name:'Google 계정으로 로그인'})).getAttribute('href')).toBe('/api/v1/auth/google/start')
})

test('관리자가 가입을 승인하고 목록에서 이용 중 상태를 확인한다',async()=>{
 const {MemberAdmin}=await import('./Administration');const {SessionProvider}=await import('./session');let approved=false;vi.stubGlobal('confirm',()=>true)
 vi.stubGlobal('fetch',async(input:RequestInfo|URL)=>{
  const p=String(input)
  if(p.endsWith('/auth/config'))return Response.json({data:{authMode:'google',accounts:[]}})
  if(p.endsWith('/auth/me'))return Response.json({data:{user:{id:'admin',role:'HR_ADMIN',status:'ACTIVE'},csrfToken:'csrf'}})
  if(p.endsWith('/approve')){approved=true;return Response.json({data:{updated:true}})}
  if(p.endsWith('/admin/members'))return Response.json({data:{members:[{id:'pending',realName:'가상 직원',email:'fake@example.org',department:'인사팀',jobLevel:'사원',role:approved?'HR_ADMIN':'MEMBER',status:approved?'ACTIVE':'PENDING'}]}})
  throw Error(p)
 })
 render(<SessionProvider><MemberAdmin/></SessionProvider>);fireEvent.click(await screen.findByRole('button',{name:'가입 승인'}));await screen.findByText('이용 중');expect(screen.queryByRole('button',{name:'가입 승인'})).toBeNull()
})

test('정지된 구글 계정은 로그인 화면에서 제한 이유를 확인한다',async()=>{
 vi.stubGlobal('fetch',async(input:RequestInfo|URL)=>String(input).endsWith('/auth/config')?Response.json({data:{authMode:'google',googleLogin:true,developmentLogin:false,accounts:[]}}):Response.json({error:{code:'ACCOUNT_SUSPENDED',message:'이용이 제한된 계정입니다.'}},{status:403}))
 render(<App/>);await screen.findByRole('alert');expect(screen.getByRole('alert').textContent).toContain('이용이 제한된 계정입니다.')
})
