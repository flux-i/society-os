import { useLayoutEffect } from 'react'
import type { DependencyList } from 'react'

// A deferred screen may start rendering before the next fragment arrives.
// Subscribe and reconcile the latest URL before paint, including first mount.
export function useFragmentSync(sync:()=>void,dependencies:DependencyList){
 useLayoutEffect(()=>{window.addEventListener('hashchange',sync);sync();return()=>window.removeEventListener('hashchange',sync)},dependencies)
}
