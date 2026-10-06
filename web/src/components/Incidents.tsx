import { useEffect, useState } from 'react'
import type { User } from '../api'
import {
  incidentLink,
  incidentStates,
  ruleStates,
  selectOptions,
  useIncidentLoad
} from '../incidents'
import type {
  IncidentDetail,
  IncidentPage,
  NoticePage,
  Rule,
  RulePage
} from '../incidents'
import { displayDate } from '../maintenance'
import { IncidentDialog, IncidentNoticeDialog } from './IncidentDetails'
import { IncidentReportForm } from './IncidentReportForm'
import { NewRule, RuleDialog } from './IncidentRules'
import { FilterSelect } from './FilterSelect'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'
type Modal =
  | {
      kind: 'case' | 'rule' | 'notice'
      id: string
    }
  | {
      kind: 'report'
      existing?: IncidentDetail
    }
  | {
      kind: 'new-rule'
      replacement?: Rule
    }
  | null
function linked(): Modal {
  for (const kind of ['case', 'rule', 'notice'] as const) {
    const id = incidentLink(kind)
    if (/^[A-Za-z0-9_-]{43}$/.test(id)) return { kind, id }
  }
  return null
}
function ConductArt() {
  return (
    <svg
      className="conduct-art"
      aria-hidden="true"
      viewBox="0 0 260 210"
      fill="none"
    >
      <ellipse cx="132" cy="177" rx="108" ry="17" fill="#e3e6d7" />
      <path
        d="M74 55c15-15 39-17 58-10 22-8 46-3 59 10v109c-17-10-40-12-59-4-19-8-42-6-58 4V55Z"
        fill="#fdfcf6"
        stroke="#b7bfa7"
        strokeWidth="2"
      />
      <path
        d="M132 48v111M90 75h25M90 91h25M90 108h21M149 75h25M149 91h25M149 108h20"
        stroke="#a8b39a"
        strokeWidth="2"
        strokeLinecap="round"
      />
      <circle cx="61" cy="142" r="31" fill="#d8e4bd" />
      <path
        d="m49 142 8 8 17-20"
        stroke="#526e4d"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
      <path
        d="M203 139c-8-13-7-32 3-43 13 12 16 27 8 41M205 171v-38"
        stroke="#7d946e"
        strokeWidth="2"
      />
      <path d="M220 132c12-15 17-16 24-16-1 14-9 25-24 26" fill="#bdcba7" />
      <circle cx="51" cy="61" r="7" fill="#cabb9e" />
    </svg>
  )
}
export function Incidents({ user }: { user: User }) {
  const [tab, setTab] = useState(() =>
      incidentLink('rule')
        ? 'RULES'
        : incidentLink('notice')
          ? 'NOTICES'
          : 'REPORTS'
    ),
    [query, setQuery] = useState(''),
    [state, setState] = useState(''),
    [page, setPage] = useState(1),
    [revision, setRevision] = useState(0),
    [modal, setModal] = useState<Modal>(linked)
  const reports = useIncidentLoad<IncidentPage>(
      '/api/incidents?' +
        new URLSearchParams({
          q: query,
          state: tab === 'REPORTS' ? state : '',
          page: String(page),
          refresh: String(revision)
        }),
      tab === 'REPORTS'
    ),
    rules = useIncidentLoad<RulePage>(
      '/api/rules?' +
        new URLSearchParams({
          q: query,
          state: tab === 'RULES' ? state : '',
          page: String(page),
          refresh: String(revision)
        }),
      tab === 'RULES'
    ),
    notices = useIncidentLoad<NoticePage>(
      '/api/incident-notices?' +
        new URLSearchParams({
          q: query,
          page: String(page),
          refresh: String(revision)
        }),
      tab === 'NOTICES'
    ),
    permission = useIncidentLoad<IncidentPage>(
      '/api/incidents?page=1&refresh=' + revision,
      tab !== 'REPORTS'
    )
  useEffect(() => {
    const update = () => {
      const next = linked()
      setModal(next)
      if (next?.kind === 'rule') setTab('RULES')
      else if (next?.kind === 'notice') setTab('NOTICES')
      else if (next?.kind === 'case') setTab('REPORTS')
    }
    window.addEventListener('hashchange', update)
    return () => window.removeEventListener('hashchange', update)
  }, [])
  const load = tab === 'RULES' ? rules : tab === 'NOTICES' ? notices : reports,
    data = load.data,
    open = (kind: 'case' | 'rule' | 'notice', id: string) => {
      setModal({ kind, id })
      window.history.replaceState(null, '', '#conduct?' + kind + '=' + id)
    },
    close = () => {
      setModal(null)
      window.history.replaceState(null, '', '#conduct')
    },
    changed = () => setRevision((value) => value + 1),
    switchTab = (value: string) => {
      setTab(value)
      setQuery('')
      setState('')
      setPage(1)
    },
    canReport =
      tab === 'REPORTS' ? reports.data?.can_report : permission.data?.can_report
  const statusLabels =
    tab === 'RULES'
      ? user.can_handle_complaints
        ? ruleStates
        : { PUBLISHED: 'Published', RETIRED: 'Retired' }
      : incidentStates
  return (
    <div className="page-enter conduct-page">
      <section className="reviews-hero conduct-hero">
        <div>
          <span className="eyebrow">CARE, WITH FAIRNESS</span>
          <h1>
            Fair rules.
            <br />
            <em>Thoughtful decisions.</em>
          </h1>
          <p>
            A private place to raise an observation, review the facts and give
            people a chance to respond.
          </p>
          <div className="incident-action-tools">
            <button
              className="button button-dark"
              disabled={!canReport}
              onClick={() => setModal({ kind: 'report' })}
            >
              Report an observation
              <Icon name="plus" />
            </button>
            {user.can_handle_complaints && (
              <button
                className="button button-outline"
                onClick={() => setModal({ kind: 'new-rule' })}
              >
                Propose a rule
                <Icon name="document" />
              </button>
            )}
          </div>
        </div>
        <ConductArt />
      </section>
      <div className="review-principles">
        <span>
          <Icon name="shield" />
          Private observations
        </span>
        <span>
          <Icon name="community" />
          Independent review
        </span>
        <span>
          <Icon name="records" />A preserved account
        </span>
      </div>
      <div
        className="detail-tabs conduct-tabs"
        role="tablist"
        aria-label="Rules and conduct"
      >
        {[
          [
            'REPORTS',
            user.can_handle_complaints ? 'Handling desk' : 'Your reports'
          ],
          ['NOTICES', 'Response notices'],
          ['RULES', 'Community rules']
        ].map(([value, label]) => (
          <button
            role="tab"
            key={value}
            aria-selected={tab === value}
            onClick={() => switchTab(value)}
          >
            {label}
          </button>
        ))}
      </div>
      <section className="registry-panel">
        <div className="registry-toolbar">
          <div>
            <h2>
              {tab === 'RULES'
                ? 'The expectations we share.'
                : tab === 'NOTICES'
                  ? 'A chance to be heard.'
                  : user.can_handle_complaints
                    ? 'The handling desk.'
                    : 'Your private reports.'}
            </h2>
            <p>
              {tab === 'RULES'
                ? 'Published supplied rules and their preserved versions.'
                : tab === 'NOTICES'
                  ? 'Notices deliberately issued for your current home relationships.'
                  : user.can_handle_complaints
                    ? 'Review the supplied facts. Decide and record the next step.'
                    : 'Only your own reports appear here, with their visible progress.'}
            </p>
          </div>
          <span className="result-count" role="status">
            {load.loading
              ? 'Opening records…'
              : load.error
                ? 'Connection unavailable'
                : `${data?.total ?? 0} ${(data?.total ?? 0) === 1 ? 'record' : 'records'}`}
          </span>
        </div>
        <div
          className={
            'filter-row conduct-filters ' +
            (tab === 'NOTICES' ? 'conduct-notice-filters' : '')
          }
        >
          <label className="search-control">
            <Icon name="search" />
            <span className="sr-only">
              Search{' '}
              {tab === 'RULES'
                ? 'community rules'
                : tab === 'NOTICES'
                  ? 'response notices'
                  : 'incident reports'}
            </span>
            <input
              type="search"
              value={query}
              onChange={(event) => {
                setQuery(event.target.value)
                setPage(1)
              }}
              maxLength={100}
              placeholder={
                tab === 'RULES'
                  ? 'Find a rule…'
                  : tab === 'NOTICES'
                    ? 'Find a notice…'
                    : 'Find a rule, home or incident date…'
              }
            />
          </label>
          {tab !== 'NOTICES' && (
            <FilterSelect
              label={
                tab === 'RULES'
                  ? 'Filter rule status'
                  : 'Filter incident status'
              }
              value={state}
              onChange={(value) => {
                setState(value)
                setPage(1)
              }}
              options={[
                { value: '', label: 'All statuses' },
                ...selectOptions(statusLabels)
              ]}
            />
          )}
          <button
            className="clear-button"
            disabled={!query && !state}
            onClick={() => {
              setQuery('')
              setState('')
              setPage(1)
            }}
          >
            Clear
            <Icon name="close" />
          </button>
        </div>
        <div aria-busy={load.loading}>
          {load.error ? (
            <div className="empty-state" role="alert">
              <h3>Let’s open that again.</h3>
              <p>{load.error}</p>
              <button className="button button-dark" onClick={load.reload}>
                Try again
                <Icon name="refresh" />
              </button>
            </div>
          ) : load.loading ? (
            <div className="home-skeleton skeleton" />
          ) : !data?.items.length ? (
            <div className="records-empty">
              <Icon name="leaf" />
              <h3>
                {query || state
                  ? 'No matching records.'
                  : tab === 'RULES'
                    ? 'A shared understanding starts here.'
                    : tab === 'NOTICES'
                      ? 'Nothing needs your response.'
                      : 'A little calm, for now.'}
              </h3>
              <p>
                {query || state
                  ? 'Try another title, home or status.'
                  : tab === 'RULES'
                    ? 'An operator can propose the society’s supplied rules for independent review.'
                    : tab === 'NOTICES'
                      ? 'A notice appears only when handlers deliberately issue it to your current home.'
                      : 'An observation stays private while the team considers its facts.'}
              </p>
            </div>
          ) : tab === 'RULES' ? (
            <div className="rule-grid">
              {rules.data?.items.map((item) => (
                <button
                  className="rule-card"
                  key={item.id}
                  onClick={() => open('rule', item.id)}
                  aria-label={'Open rule ' + item.title}
                >
                  <span className="maintenance-card-top">
                    <span className="fund-state">{ruleStates[item.state]}</span>
                    <Icon name="arrow" />
                  </span>
                  <span className="rule-book-mark">
                    <Icon name="document" />
                  </span>
                  <h3>{item.title}</h3>
                  <p>{item.text}</p>
                  <span className="rule-card-foot">
                    Effective {displayDate(item.effective_from)}
                    <small>
                      {item.fine_permitted
                        ? 'Separate fine proposals permitted'
                        : 'Review without fine permission'}
                    </small>
                  </span>
                </button>
              ))}
            </div>
          ) : tab === 'NOTICES' ? (
            <div className="incident-list">
              {notices.data?.items.map((item) => (
                <button
                  className="incident-list-card"
                  key={item.id}
                  onClick={() => open('notice', item.id)}
                  aria-label={'Open response notice ' + item.title}
                >
                  <span className="incident-card-mark">
                    <Icon name="community" />
                  </span>
                  <span className="incident-card-story">
                    <span className="eyebrow">HOME {item.home}</span>
                    <strong>{item.title}</strong>
                    <small>Response by {displayDate(item.response_by)}</small>
                  </span>
                  <Icon name="arrow" />
                </button>
              ))}
            </div>
          ) : (
            <div className="incident-list">
              {reports.data?.items.map((item) => (
                <button
                  className="incident-list-card"
                  key={item.id}
                  onClick={() => open('case', item.id)}
                  aria-label={
                    'Open incident ' +
                    item.rule_title +
                    ' ' +
                    item.home +
                    ' ' +
                    item.id.slice(0, 6)
                  }
                >
                  <span className="incident-card-mark">
                    <Icon name="shield" />
                  </span>
                  <span className="incident-card-story">
                    <span className="eyebrow">
                      HOME {item.home} · {displayDate(item.incident_date)}
                    </span>
                    <strong>{item.rule_title}</strong>
                    <small>
                      Case {item.id.slice(0, 6)}
                      {item.picture_id ? ' · Picture attached' : ''}
                    </small>
                  </span>
                  <span className="fund-state">
                    {incidentStates[item.state]}
                  </span>
                  <Icon name="arrow" />
                </button>
              ))}
            </div>
          )}
        </div>
        {data && !load.error && !load.loading && data.total > 12 && (
          <PageControls
            label={
              tab === 'RULES'
                ? 'community rules'
                : tab === 'NOTICES'
                  ? 'response notices'
                  : 'incident reports'
            }
            page={data.page}
            total={data.total}
            size={12}
            onPage={setPage}
          />
        )}
      </section>
      {modal?.kind === 'new-rule' && (
        <NewRule
          key={modal.replacement?.id ?? 'new-rule'}
          replacement={modal.replacement}
          onClose={close}
          onSaved={(id) => {
            switchTab('RULES')
            changed()
            open('rule', id)
          }}
        />
      )}
      {modal?.kind === 'report' && (
        <IncidentReportForm
          key={modal.existing?.id ?? 'new-report'}
          existing={modal.existing}
          onClose={close}
          onSaved={(id) => {
            switchTab('REPORTS')
            changed()
            open('case', id)
          }}
        />
      )}
      {modal?.kind === 'rule' && (
        <RuleDialog
          key={'rule-' + modal.id}
          id={modal.id}
          user={user}
          onClose={close}
          onSaved={changed}
          onReplace={(replacement) =>
            setModal({ kind: 'new-rule', replacement })
          }
        />
      )}{' '}
      {modal?.kind === 'case' && (
        <IncidentDialog
          key={'case-' + modal.id}
          id={modal.id}
          user={user}
          onClose={close}
          onSaved={changed}
          onRevise={(existing) => setModal({ kind: 'report', existing })}
          onNotice={(id) => open('notice', id)}
        />
      )}{' '}
      {modal?.kind === 'notice' && (
        <IncidentNoticeDialog
          key={'notice-' + modal.id}
          id={modal.id}
          onClose={close}
          onSaved={changed}
        />
      )}
    </div>
  )
}
