import { useEffect } from 'react'
import { userAccessScope } from './api'
import type { User } from './api'
import { usePortalConnection } from './pwa'

// Optional tools load only in a browser exposing the native API and after MFA.
export function useSocietyTools(user:User,openHome:(id:string)=>void){
 const connection=usePortalConnection()
 const scope=userAccessScope(user)
 useEffect(()=>{
  if(user.mfa_pending||connection.connection!=='online'||connection.pendingSignout||!(document as Document&{modelContext?:unknown}).modelContext)return
  let disposed=false,cleanup:(()=>void)|undefined
  void import('./webmcp-register').then(({registerSocietyTools})=>{if(!disposed)cleanup=registerSocietyTools(user,openHome)}).catch(()=>{/* Experimental availability never blocks the portal. */})
  return()=>{disposed=true;cleanup?.()}
 },[user.id,user.mfa_pending,scope,openHome,connection.connection,connection.pendingSignout])
}
