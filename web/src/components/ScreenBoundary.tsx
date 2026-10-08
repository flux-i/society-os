import { Component } from 'react'
import type { ReactNode } from 'react'
import { Icon } from './Icon'

export function ScreenLoading(){return <section className="screen-state" role="status" aria-live="polite" aria-busy="true"><span className="chapter-icon"><Icon name="leaf"/></span><span className="eyebrow">A LITTLE MOMENT</span><h1>Opening this screen…</h1><p>Your workspace stays close while the next view opens.</p><div className="screen-loading-mark" aria-hidden="true"/></section>}

export class ScreenBoundary extends Component<{children:ReactNode},{failed:boolean}>{
 state={failed:false}
 static getDerivedStateFromError(){return{failed:true}}
 render(){return this.state.failed?<section className="screen-state screen-state-error" role="alert"><span className="chapter-icon"><Icon name="leaf"/></span><span className="eyebrow">LET’S TRY THAT AGAIN</span><h1>This screen could not be opened.</h1><p>Reload this screen to try again. Your saved records stay on record.</p><div className="form-actions"><button type="button" className="button button-dark" onClick={()=>window.location.reload()}>Reload this screen<Icon name="refresh"/></button><a className="button button-quiet" href="#overview">Back to overview<Icon name="arrow"/></a></div></section>:this.props.children}
}
