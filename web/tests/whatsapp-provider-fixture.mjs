// Official protocol fixtures use fictional identities and credentials only.
// This server is created solely by isolated browser QA, never by the product.
import { createServer } from 'node:http'
import { createHmac } from 'node:crypto'
import { mkdirSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'

export async function createWhatsAppFixture(root, portal) {
  const access='FICTIONAL_BROWSER_ACCESS_TOKEN_123456789',secret='FICTIONAL_BROWSER_APP_SECRET_123456789'
  const verify='FICTIONAL_BROWSER_VERIFICATION_TOKEN_123456789'
  const originals={
    '444444':{id:'444444',name:'society_notice',language:'en',status:'APPROVED',category:'MARKETING',components:[{type:'BODY',text:'Your society has shared an update. Open it securely: {{1}}'}]},
    '555555':{id:'555555',name:'society_receipt',language:'en',status:'APPROVED',category:'UTILITY',components:[{type:'BODY',text:'Your society receipt is ready. Open it securely: {{1}}'}]},
    '666666':{id:'666666',name:'society_statement',language:'en',status:'APPROVED',category:'UTILITY',components:[{type:'BODY',text:'A published society statement is ready. Open it securely: {{1}}'}]},
  }
  let templates=structuredClone(originals),mode='ACCEPTED',sequence=0
  const sends=[],proofs=new Map()
  const read=async request=>{const chunks=[];let size=0;for await(const chunk of request){size+=chunk.length;if(size>65536)throw new Error('fixture input too large');chunks.push(chunk)}return JSON.parse(Buffer.concat(chunks).toString()||'{}')}
  const json=(response,code,data)=>{response.writeHead(code,{'Content-Type':'application/json'});response.end(JSON.stringify(data))}
  const server=createServer(async(request,response)=>{
    try {
      const url=new URL(request.url,'http://127.0.0.1')
      if(url.pathname==='/_control'&&request.method==='POST') {
        const input=await read(request)
        if(input.reset){templates=structuredClone(originals);mode='ACCEPTED'}
        if(input.mode)mode=input.mode
        if(input.template)Object.assign(templates[input.template.id],input.template)
        json(response,200,{mode,send_count:sends.length});return
      }
      if(url.pathname==='/_status'&&request.method==='GET'){json(response,200,{mode,send_count:sends.length,sends});return}
      if(url.pathname==='/_proof'&&request.method==='POST') {
        const input=await read(request),send=sends.find(value=>value.id===input.id)
        if(!send){json(response,404,{error:'unknown_fixture_send'});return}
        const key=send.id+':'+input.state
        if(!proofs.has(key))proofs.set(key,{object:'whatsapp_business_account',entry:[{id:'111111',changes:[{field:'messages',value:{messaging_product:'whatsapp',metadata:{phone_number_id:'222222'},statuses:[{id:send.id,status:input.state,timestamp:String(Math.floor(Date.now()/1000)),recipient_id:send.request.to.replace(/^\+/,''),biz_opaque_callback_data:send.request.biz_opaque_callback_data,pricing:{category:'marketing',billable:true}}]}}]}]})
        const payload=JSON.stringify(proofs.get(key)),signature=createHmac('sha256',secret).update(payload).digest('hex')
        const callback=await fetch(portal+'/providers/whatsapp/webhook',{method:'POST',headers:{'Content-Type':'application/json','X-Hub-Signature-256':'sha256='+(input.bad_signature?'0'.repeat(64):signature)},body:payload})
        json(response,200,{callback_status:callback.status});return
      }
      if(request.headers.authorization!=='Bearer '+access){json(response,401,{error:{code:190}});return}
      const templateId=url.pathname.match(/^\/v26\.0\/(444444|555555|666666)$/)?.[1]
      if(request.method==='GET'&&templateId){json(response,200,templates[templateId]);return}
      if(request.method==='POST'&&url.pathname==='/v26.0/222222/messages') {
        const input=await read(request),id='wamid.FICTIONAL_BROWSER_'+String(++sequence).padStart(6,'0')
        sends.push({id,request:input,authorization_valid:true,mode})
        if(mode==='DROP_RESPONSE'){request.socket.destroy();return}
        if(mode==='RATE_LIMITED'){response.setHeader('Retry-After','60');json(response,429,{error:{code:130429}});return}
        if(mode==='REJECTED'){json(response,400,{error:{code:131026}});return}
        if(mode==='MALFORMED'){response.writeHead(200,{'Content-Type':'application/json'});response.end('{"messages":');return}
        if(mode==='SERVER_ERROR'){json(response,500,{error:{code:2}});return}
        json(response,200,{messaging_product:'whatsapp',contacts:[{input:input.to,wa_id:input.to.replace(/^\+/, '')}],messages:[{id}]});return
      }
      json(response,404,{error:{code:100}})
    } catch {json(response,400,{error:{code:100}})}
  })
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)})
  const origin='http://127.0.0.1:'+server.address().port
  const directory=join(root,'provider-keys');mkdirSync(directory,{mode:0o700})
  const config=join(directory,'whatsapp.json')
  writeFileSync(config,JSON.stringify({origin,api_version:'v26.0',account_id:'111111',phone_id:'222222',messaging_account_id:'333333',access_token:access,app_secret:secret,verify_token:verify,templates:{NOTICE:{id:'444444',name:'society_notice',language:'en'},RECEIPT:{id:'555555',name:'society_receipt',language:'en'},STATEMENT:{id:'666666',name:'society_statement',language:'en'}}}),{mode:0o600})
  return {origin,config,close:async()=>{server.closeAllConnections();await new Promise(resolve=>server.close(resolve))}}
}
