import type { AreaHome } from './community'
export { useFundLoad as useMeetingLoad } from './collections'
export { useFineWrite as useMeetingWrite } from './fines'
export type MeetingSnapshot = {
 version:number;action:string;title:string;body:string;location:string;scope:string;building_code:string;homes:AreaHome[];
 start_at:number;end_at:number;held_at:number;minutes?:string;update_text?:string;ack_required:boolean;ack_deadline:number;
 submitted_by?:string;submitted_at?:number;reason?:string
}
export type MeetingResource = {
 id:string;version:number;state:string;published_at:number;snapshot:MeetingSnapshot;published?:MeetingSnapshot;
 acknowledgement:{required:boolean;deadline:number;fingerprint:string;acknowledged:boolean;at:number;can_acknowledge:boolean};
 can_revise:boolean;can_decide:boolean;can_cancel:boolean;can_prepare_minutes:boolean;can_prepare_cancellation:boolean;can_withdraw:boolean
}
export type MeetingDetail = MeetingResource & {events?:{version:number;action:string;actor:string;reason:string;at:number;snapshot:MeetingSnapshot}[];event_total?:number;event_page?:number;page_size:number}
export type MeetingPage = {items:MeetingResource[];total:number;page:number;page_size:number;counts:Record<string,number>}
export type MeetingResponses = {
 publication_version:number;expected:number;acknowledged:number;outstanding:number;historical_total:number;page:number;history_page:number;page_size:number;
 items:{resident_id:string;name:string;acknowledged:boolean;at:number;homes:AreaHome[]}[];
 history:{version:number;resident_id:string;name:string;at:number;homes:AreaHome[]}[]
}
export const meetingStates:Record<string,string>={UPCOMING:'Upcoming meeting',PAST:'Scheduled time passed',MINUTES:'Minutes published',CANCELLED:'Meeting cancelled',PENDING:'Awaiting separate review',DECLINED:'Proposal declined',PROPOSAL_CANCELLED:'Proposal cancelled',WITHDRAWN:'Publication withdrawn'}
export const meetingActions:Record<string,string>={AGENDA:'Meeting agenda',MINUTES:'Supplied minutes',CANCEL:'Supplied cancellation',WITHDRAW:'Publication withdrawal'}
export const meetingDate=(at:number,part:'day'|'month'|'time')=>new Intl.DateTimeFormat('en-IN',{timeZone:'Asia/Kolkata',...(part==='day'?{day:'2-digit'}:part==='month'?{month:'short'}:{hour:'numeric',minute:'2-digit'})}).format(new Date(at*1000))
