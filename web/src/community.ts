export { useFundLoad as useCommunityLoad } from './collections'
export { useFineWrite as useCommunityWrite } from './fines'
export type AreaHome = {id:string;label:string}
export type CommunitySnapshot = {version:number;kind:string;action:string;title:string;body:string;service:string;phone?:string;availability?:string;attestation?:string;scope:string;building_code:string;homes:AreaHome[];start_at:number;estimated_end:number;resolved_at:number;update_text?:string;submitted_by?:string;submitted_at?:number;reason?:string}
export type CommunityResource = {id:string;version:number;state:string;published_at:number;snapshot:CommunitySnapshot;published?:CommunitySnapshot;can_revise:boolean;can_decide:boolean;can_cancel:boolean;can_resolve:boolean;can_withdraw:boolean}
export type CommunityDetail = CommunityResource & {events?:{version:number;action:string;actor:string;reason:string;at:number;snapshot:CommunitySnapshot}[];event_total?:number;event_page?:number;page_size:number}
export type CommunityPage = {items:CommunityResource[];total:number;page:number;page_size:number;counts:Record<string,number>}
export type CommunityOptions = {homes:AreaHome[];buildings:AreaHome[];area_key:string}
export const serviceLabels:Record<string,string>={WATER:'Water',POWER:'Power',LIFT:'Lift',OTHER:'Other services'}
export const communityStates:Record<string,string>={PENDING:'Awaiting separate review',AVAILABLE:'Published contact',ACTIVE:'Service interrupted',UPDATE_NEEDED:'Estimate passed · update needed',PLANNED:'Planned interruption',RESOLVED:'Service restored',WITHDRAWN:'Publication withdrawn',DECLINED:'Proposal declined',CANCELLED:'Proposal cancelled'}
export const communityActions:Record<string,string>={PROPOSED:'Proposal prepared',REVISED:'Replacement proposal prepared',APPROVED:'Exact proposal approved',DECLINED:'Proposal declined',CANCELLED:'Proposal cancelled'}
export const communityTime=(at:number)=>at?new Intl.DateTimeFormat('en-IN',{day:'numeric',month:'short',year:'numeric',hour:'numeric',minute:'2-digit',timeZone:'Asia/Kolkata'}).format(new Date(at*1000))+' IST':'Not supplied'
export const communityLocal=(at:number)=>new Date((at+19800)*1000).toISOString().slice(0,16)
export const communityInstant=(local:string)=>local?Math.floor(new Date(local+'+05:30').getTime()/1000):0
export const communityLink=(key:string)=>new URLSearchParams(window.location.hash.split('?')[1]??'').get(key)??''
