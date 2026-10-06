import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import type { User } from '../api'
import { ruleStates, useIncidentLoad, useIncidentWrite } from '../incidents'
import type { Rule } from '../incidents'
import { displayDate } from '../maintenance'
import { careTime } from '../upkeep'
import { FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'
import {
  IncidentCheck,
  IncidentFeedback,
  IncidentHistory,
  IncidentUnavailable
} from './IncidentShared'
import { Icon } from './Icon'
export function RuleText({ rule }: { rule: Rule }) {
  return (
    <section className="rule-paper">
      <span className="eyebrow">SUPPLIED COMMUNITY POLICY</span>
      <h3>{rule.title}</h3>
      <p className="preserve-lines">{rule.text}</p>
      <div className="rule-dates">
        <span>
          Effective from<strong>{displayDate(rule.effective_from)}</strong>
        </span>
        <span>
          Effective until
          <strong>
            {rule.effective_until
              ? displayDate(rule.effective_until)
              : 'No end supplied'}
          </strong>
        </span>
      </div>
      <p className="rule-reference">
        Authority reference · {rule.policy_reference}
      </p>
      <p className="form-help">
        {rule.fine_permitted
          ? 'Supplied policy permits a separately reviewed fine proposal.'
          : 'No fine permission is supplied with this rule.'}
      </p>
    </section>
  )
}
export function NewRule({
  replacement,
  onClose,
  onSaved
}: {
  replacement?: Rule
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const [title, setTitle] = useState(replacement?.title ?? ''),
    [text, setText] = useState(replacement?.text ?? ''),
    [policy, setPolicy] = useState(replacement?.policy_reference ?? ''),
    [from, setFrom] = useState(replacement?.effective_from ?? ''),
    [until, setUntil] = useState(''),
    [fine, setFine] = useState(replacement?.fine_permitted ?? false),
    [review, setReview] = useState(false),
    [checked, setChecked] = useState(false),
    writer = useIncidentWrite(),
    heading = useRef<HTMLHeadingElement>(null)
  useEffect(() => {
    if (review) heading.current?.focus()
  }, [review])
  const save = async (event: FormEvent) => {
    event.preventDefault()
    if (!checked) return
    const result = await writer.send('/api/rules', {
      title,
      text,
      policy_reference: policy,
      effective_from: from,
      effective_until: until,
      fine_permitted: fine,
      replaces_id: replacement?.id ?? ''
    })
    if (result) onSaved(result.id)
  }
  const frozen = {
    title,
    text,
    policy_reference: policy,
    effective_from: from,
    effective_until: until,
    fine_permitted: fine
  } as Rule
  return (
    <PortalDialog
      titleId="rule-title"
      closeLabel="Close new rule"
      className="conduct-dialog"
      onClose={onClose}
      busy={writer.busy}
    >
      <div className="dialog-scroll detail-body incident-dialog">
        <span className="eyebrow">
          {replacement
            ? 'A PRESERVED REPLACEMENT'
            : 'CLEAR EXPECTATIONS, SHARED'}
        </span>
        <h2 id="rule-title" tabIndex={-1} ref={heading}>
          {review ? (
            'Review this rule.'
          ) : (
            <>
              A rule,
              <br />
              <em>made clear.</em>
            </>
          )}
        </h2>
        <p className="detail-intro">
          {replacement
            ? 'A different operator must approve this replacement. Its original stays with earlier cases.'
            : 'Supply the society’s policy and authority. A different operator must publish it.'}
        </p>
        {review ? (
          <form className="portal-form" onSubmit={save}>
            <RuleText rule={frozen} />
            {replacement && (
              <p className="form-help">
                Publication deliberately retires the current version of “
                {replacement.title}”. Earlier reports retain that original.
              </p>
            )}
            <IncidentCheck
              checked={checked}
              onChange={setChecked}
              disabled={writer.busy || writer.locked}
            >
              I reviewed the supplied rule text, dates and authority.
            </IncidentCheck>
            <IncidentFeedback writer={writer} />
            <div className="incident-action-tools">
              <button
                type="button"
                className="button button-outline"
                disabled={writer.busy || writer.locked}
                onClick={() => {
                  setReview(false)
                  setChecked(false)
                }}
              >
                Edit rule
              </button>
              <button
                className="button button-dark"
                disabled={writer.busy || !checked}
              >
                {writer.busy
                  ? 'Submitting…'
                  : writer.locked
                    ? 'Retry this rule'
                    : 'Submit rule for review'}
                <Icon name="arrow" />
              </button>
            </div>
          </form>
        ) : (
          <form
            className="portal-form"
            onSubmit={(event) => {
              event.preventDefault()
              setReview(true)
            }}
          >
            <fieldset className="records-fieldset">
              <label>
                Rule title
                <input
                  value={title}
                  onChange={(event) => setTitle(event.target.value)}
                  required
                  minLength={5}
                  maxLength={120}
                />
              </label>
              <label>
                Supplied rule text
                <textarea
                  value={text}
                  onChange={(event) => setText(event.target.value)}
                  required
                  minLength={10}
                  maxLength={4000}
                  rows={5}
                />
              </label>
              <label>
                Policy / authority reference
                <input
                  value={policy}
                  onChange={(event) => setPolicy(event.target.value)}
                  required
                  minLength={5}
                  maxLength={300}
                />
              </label>
              <div className="rule-dates-input">
                <label>
                  Effective from
                  <input
                    type="date"
                    value={from}
                    onChange={(event) => setFrom(event.target.value)}
                    required
                    min="1900-01-01"
                    max="2100-12-31"
                  />
                </label>
                <label>
                  Effective until (optional)
                  <input
                    type="date"
                    value={until}
                    onChange={(event) => setUntil(event.target.value)}
                    min={from || '1900-01-01'}
                    max="2100-12-31"
                  />
                </label>
              </div>
              <IncidentCheck checked={fine} onChange={setFine}>
                The supplied policy permits proposing a fine.
              </IncidentCheck>
            </fieldset>
            <p className="form-help">
              Publishing a rule creates no charge. Amounts and issuance require
              their own later decision.
            </p>
            <button className="button button-dark">
              Review this rule
              <Icon name="arrow" />
            </button>
          </form>
        )}
      </div>
    </PortalDialog>
  )
}
export function RuleDialog({
  id,
  user,
  onClose,
  onSaved,
  onReplace
}: {
  id: string
  user: User
  onClose: () => void
  onSaved: () => void
  onReplace: (rule: Rule) => void
}) {
  const [page, setPage] = useState(1),
    load = useIncidentLoad<Rule>(
      '/api/rules/' + encodeURIComponent(id) + '?event_page=' + page
    ),
    [action, setAction] = useState(''),
    [reason, setReason] = useState(''),
    [checked, setChecked] = useState(false),
    writer = useIncidentWrite(),
    x = load.data
  useEffect(() => {
    setChecked(false)
    setAction('')
  }, [x?.version])
  const reload = () => {
      writer.reset()
      load.reload()
    },
    save = async (event: FormEvent) => {
      event.preventDefault()
      if (!x || !checked) return
      const saved = await writer.send(
        '/api/rules/' + encodeURIComponent(id) + '/actions',
        { version: x.version, action, reason }
      )
      if (saved) {
        setAction('')
        setReason('')
        setChecked(false)
        load.reload()
        onSaved()
      }
    }
  const actions = x
    ? x.state === 'PENDING'
      ? x.author_id === user.id
        ? ['WITHDRAWN']
        : ['PUBLISHED', 'DECLINED']
      : x.state === 'PUBLISHED'
        ? ['RETIRED']
        : []
    : []
  return (
    <PortalDialog
      titleId="rule-detail-title"
      closeLabel="Close rule details"
      className="conduct-dialog"
      onClose={onClose}
      busy={writer.busy}
    >
      <div className="dialog-scroll detail-body incident-dialog">
        <span className="eyebrow">COMMUNITY RULES</span>
        <h2 id="rule-detail-title">
          {load.error
            ? 'Rule unavailable'
            : load.loading
              ? 'Opening this rule…'
              : x?.title}
        </h2>
        <IncidentUnavailable
          loading={load.loading}
          error={load.error}
          onRetry={load.reload}
        >
          {x && (
            <>
              <div className="incident-detail-tools">
                <span className="fund-state">{ruleStates[x.state]}</span>
                <button
                  type="button"
                  className="text-link"
                  disabled={writer.busy || writer.locked}
                  onClick={load.reload}
                >
                  Refresh rule
                  <Icon name="refresh" />
                </button>
              </div>
              <RuleText rule={x} />
              <p className="form-help">
                Recorded {careTime(x.created_at)} · Version {x.version}
              </p>
              {user.can_handle_complaints && x.state === 'PUBLISHED' && (
                <button
                  className="button button-outline"
                  disabled={writer.busy || writer.locked}
                  onClick={() => onReplace(x)}
                >
                  Prepare replacement
                  <Icon name="document" />
                </button>
              )}
              {user.can_handle_complaints &&
                x.state === 'PENDING' &&
                x.author_id === user.id && (
                  <p className="form-help">
                    A different operator must publish your proposed rule.
                  </p>
                )}
              {user.can_handle_complaints && actions.length > 0 && (
                <form className="portal-form incident-decision" onSubmit={save}>
                  <h3>The next decision.</h3>
                  <fieldset
                    className="records-fieldset"
                    disabled={writer.busy || writer.locked}
                  >
                    <label>
                      Rule decision
                      <FormSelect
                        label="Rule decision"
                        value={action}
                        onChange={(value) => {
                          setAction(value)
                          setChecked(false)
                        }}
                        required
                        options={[
                          { value: '', label: 'Choose a decision…' },
                          ...actions.map((value) => ({
                            value,
                            label:
                              value === 'PUBLISHED'
                                ? 'Publish rule'
                                : value === 'RETIRED'
                                  ? 'Retire rule'
                                  : value === 'DECLINED'
                                    ? 'Decline rule'
                                    : 'Withdraw rule'
                          }))
                        ]}
                      />
                    </label>
                    <label>
                      Reason for rule decision
                      <textarea
                        value={reason}
                        onChange={(event) => setReason(event.target.value)}
                        minLength={10}
                        maxLength={2000}
                        required
                        rows={3}
                      />
                    </label>
                    <IncidentCheck checked={checked} onChange={setChecked}>
                      I reviewed this decision and the supplied policy.
                    </IncidentCheck>
                  </fieldset>
                  <IncidentFeedback writer={writer} onReload={reload} />
                  <button
                    className="button button-dark"
                    disabled={writer.busy || !checked || !action}
                  >
                    {writer.busy
                      ? 'Saving decision…'
                      : writer.locked
                        ? 'Retry rule decision'
                        : action === 'PUBLISHED'
                          ? 'Publish reviewed rule'
                          : 'Save rule decision'}
                    <Icon name="check" />
                  </button>
                </form>
              )}
              {x.events && (
                <IncidentHistory
                  events={x.events}
                  total={x.event_total ?? 0}
                  page={x.event_page ?? 1}
                  onPage={setPage}
                  disabled={writer.busy || writer.locked}
                />
              )}
            </>
          )}
        </IncidentUnavailable>
      </div>
    </PortalDialog>
  )
}
