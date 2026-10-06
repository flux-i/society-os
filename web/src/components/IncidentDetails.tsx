import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import type { User } from '../api'
import {
  incidentActions,
  incidentStates,
  useIncidentLoad,
  useIncidentWrite
} from '../incidents'
import type {
  Incident,
  IncidentDetail,
  IncidentNotice,
  IncidentPage
} from '../incidents'
import { displayDate } from '../maintenance'
import { careTime } from '../upkeep'
import { FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'
import { PageControls } from './Maintenance'
import {
  IncidentCheck,
  IncidentFeedback,
  IncidentHistory,
  IncidentPicture,
  IncidentReviewFacts,
  IncidentUnavailable
} from './IncidentShared'
import { RuleText } from './IncidentRules'
import { Icon } from './Icon'
export function IncidentDialog({
  id,
  user,
  onClose,
  onSaved,
  onRevise,
  onNotice
}: {
  id: string
  user: User
  onClose: () => void
  onSaved: () => void
  onRevise: (item: IncidentDetail) => void
  onNotice: (id: string) => void
}) {
  const [page, setPage] = useState(1),
    load = useIncidentLoad<IncidentDetail>(
      '/api/incidents/' + encodeURIComponent(id) + '?event_page=' + page
    ),
    writer = useIncidentWrite(),
    [action, setAction] = useState(''),
    [reason, setReason] = useState(''),
    [title, setTitle] = useState(''),
    [body, setBody] = useState(''),
    [responseBy, setResponseBy] = useState(''),
    [duplicate, setDuplicate] = useState(''),
    [duplicateQuery, setDuplicateQuery] = useState(''),
    [duplicatePage, setDuplicatePage] = useState(1),
    [checked, setChecked] = useState(false),
    x = load.data
  const [selectedOriginal, setSelectedOriginal] = useState<Incident | null>(
    null
  )
  const duplicates = useIncidentLoad<IncidentPage>(
    '/api/incidents?' +
      new URLSearchParams({ q: duplicateQuery, page: String(duplicatePage) }),
    action === 'DUPLICATE'
  )
  const pageCandidates = duplicates.data?.items ?? []
  const retainedCandidates =
    selectedOriginal &&
    !pageCandidates.some((item) => item.id === selectedOriginal.id)
      ? [selectedOriginal, ...pageCandidates]
      : pageCandidates
  const candidates = retainedCandidates.filter(
    (item) =>
      item.id !== id &&
      item.flat_id === x?.flat_id &&
      item.rule_id === x?.rule_id &&
      !item.duplicate_of &&
      item.state !== 'WITHDRAWN'
  )
  const own = x?.reporter_id === user.id,
    open = x && ['REPORTED', 'NEEDS_INFO', 'UNDER_REVIEW'].includes(x.state),
    choices: string[] = []
  if (x && x.can_participate) {
    if (user.can_handle_complaints) {
      choices.push('NOTE')
      if (!own) {
        if (open)
          choices.push(
            'UNDER_REVIEW',
            'NEEDS_INFO',
            'SUBSTANTIATED',
            'DISMISSED',
            'DUPLICATE'
          )
        else if (['SUBSTANTIATED', 'DISMISSED', 'WITHDRAWN'].includes(x.state))
          choices.push('REOPEN')
        if (x.notice_id) choices.push('REMOVE_NOTICE')
        else if (open || x.state === 'SUBSTANTIATED')
          choices.push('ISSUE_NOTICE')
      }
    }
    if (own && open) choices.push('WITHDRAWN')
  }
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
      if (
        !x ||
        !checked ||
        (action === 'DUPLICATE' && (duplicates.loading || !!duplicates.error))
      )
        return
      const saved = await writer.send(
        '/api/incidents/' + encodeURIComponent(id) + '/actions',
        {
          version: x.version,
          action,
          reason,
          duplicate_of: action === 'DUPLICATE' ? duplicate : '',
          title: action === 'ISSUE_NOTICE' ? title : '',
          body: action === 'ISSUE_NOTICE' ? body : '',
          response_by: action === 'ISSUE_NOTICE' ? responseBy : ''
        }
      )
      if (saved) {
        setAction('')
        setReason('')
        setTitle('')
        setBody('')
        setChecked(false)
        load.reload()
        onSaved()
      }
    }
  return (
    <PortalDialog
      titleId="incident-detail-title"
      closeLabel="Close incident details"
      className="conduct-dialog"
      onClose={onClose}
      busy={writer.busy}
    >
      <div className="dialog-scroll detail-body incident-dialog">
        <span className="eyebrow">PRIVATE REPORT · INDEPENDENT REVIEW</span>
        <h2 id="incident-detail-title">
          {load.error
            ? 'Report unavailable'
            : load.loading
              ? 'Opening this report…'
              : x?.rule_title}
        </h2>
        <IncidentUnavailable
          loading={load.loading}
          error={load.error}
          onRetry={load.reload}
        >
          {x && (
            <>
              <div className="incident-detail-tools">
                <span
                  className={
                    'fund-state incident-state-' + x.state.toLowerCase()
                  }
                >
                  {incidentStates[x.state]}
                </span>
                <button
                  className="text-link"
                  disabled={writer.busy || writer.locked}
                  onClick={load.reload}
                >
                  Refresh report
                  <Icon name="refresh" />
                </button>
              </div>
              <IncidentReviewFacts>
                <span>
                  Tagged home<strong>{x.home}</strong>
                </span>
                <span>
                  Incident date<strong>{displayDate(x.incident_date)}</strong>
                </span>
              </IncidentReviewFacts>
              <RuleText rule={x.rule} />
              <section>
                <h3>The supplied observation</h3>
                <p className="preserve-lines">{x.comment}</p>
              </section>
              {x.picture_id && x.can_participate && (
                <IncidentPicture id={x.picture_id} original />
              )}
              {x.picture_id && !x.can_participate && (
                <p className="form-help">
                  Your retained textual history remains available. Evidence
                  access ended with your society relationship.
                </p>
              )}
              <p className="incident-no-charge">
                <Icon name="shield" />
                This report and its review create no charge or receipt.
              </p>
              {x.duplicate_of && (
                <p className="form-help">
                  Linked duplicate ·{' '}
                  <button
                    className="text-link"
                    disabled={writer.busy || writer.locked}
                    onClick={() => {
                      window.location.hash = 'conduct?case=' + x.duplicate_of
                    }}
                  >
                    Open retained original
                    <Icon name="arrow" />
                  </button>
                </p>
              )}
              {own &&
                x.can_participate &&
                ['REPORTED', 'NEEDS_INFO'].includes(x.state) && (
                  <button
                    className="button button-outline"
                    disabled={writer.busy || writer.locked}
                    onClick={() => onRevise(x)}
                  >
                    Revise your report
                    <Icon name="document" />
                  </button>
                )}
              {user.can_handle_complaints &&
                x.notices &&
                x.notices.length > 0 && (
                  <section className="incident-notice-history">
                    <h3>Response notices</h3>
                    <p>
                      {x.notice_total} retained{' '}
                      {x.notice_total === 1 ? 'notice' : 'notices'} ·{' '}
                      {x.response_total}{' '}
                      {x.response_total === 1 ? 'response' : 'responses'}
                    </p>
                    <div>
                      {x.notices.map((notice) => (
                        <button
                          className="incident-notice-link"
                          key={notice.id}
                          disabled={writer.busy || writer.locked}
                          onClick={() => onNotice(notice.id)}
                        >
                          <span>
                            <strong>{notice.title}</strong>
                            <small>
                              {notice.home} ·{' '}
                              {notice.active ? 'Current' : 'Removed'} · version{' '}
                              {notice.version}
                            </small>
                          </span>
                          <Icon name="arrow" />
                        </button>
                      ))}
                    </div>
                    {(x.notice_total ?? 0) > 20 && (
                      <p className="form-help">
                        Showing the latest 20 retained notices. Earlier
                        immutable notices remain in the case record.
                      </p>
                    )}
                  </section>
                )}
              {choices.length > 0 && (
                <form className="portal-form incident-decision" onSubmit={save}>
                  <h3>
                    {user.can_handle_complaints
                      ? 'Handle this fairly.'
                      : 'Your next step.'}
                  </h3>
                  <fieldset
                    className="records-fieldset"
                    disabled={writer.busy || writer.locked}
                  >
                    <label>
                      Incident decision
                      <FormSelect
                        label="Incident decision"
                        value={action}
                        onChange={(value) => {
                          setAction(value)
                          setChecked(false)
                        }}
                        required
                        options={[
                          { value: '', label: 'Choose the next step…' },
                          ...choices.map((value) => ({
                            value,
                            label: incidentActions[value]
                          }))
                        ]}
                      />
                    </label>
                    {action === 'DUPLICATE' && (
                      <>
                        <label>
                          Find the original case
                          <input
                            type="search"
                            value={duplicateQuery}
                            onChange={(event) => {
                              setDuplicateQuery(event.target.value)
                              setDuplicatePage(1)
                            }}
                            maxLength={100}
                          />
                        </label>
                        {duplicates.error ? (
                          <div className="form-error" role="alert">
                            <p>{duplicates.error}</p>
                            <button
                              type="button"
                              className="text-link"
                              onClick={duplicates.reload}
                            >
                              Retry original cases
                              <Icon name="refresh" />
                            </button>
                          </div>
                        ) : (
                          <label>
                            Retained original case
                            <FormSelect
                              label="Retained original case"
                              value={duplicate}
                              onChange={(value) => {
                                setDuplicate(value)
                                setSelectedOriginal(
                                  candidates.find(
                                    (item) => item.id === value
                                  ) ?? null
                                )
                                setChecked(false)
                              }}
                              required
                              disabled={duplicates.loading}
                              options={[
                                {
                                  value: '',
                                  label: duplicates.loading
                                    ? 'Opening cases…'
                                    : candidates.length
                                      ? 'Choose the same observation…'
                                      : 'No matching case on this page'
                                },
                                ...candidates.map((item) => ({
                                  value: item.id,
                                  label:
                                    item.home +
                                    ' · ' +
                                    displayDate(item.incident_date) +
                                    ' · ' +
                                    item.id.slice(0, 6)
                                }))
                              ]}
                            />
                          </label>
                        )}
                        {duplicates.data && duplicates.data.total > 12 && (
                          <PageControls
                            label="original cases"
                            page={duplicates.data.page}
                            total={duplicates.data.total}
                            size={12}
                            onPage={setDuplicatePage}
                            disabled={duplicates.loading}
                          />
                        )}
                      </>
                    )}
                    {action === 'ISSUE_NOTICE' && (
                      <>
                        <p className="form-help">
                          The current home’s residents will see only this
                          wording, the frozen rule/date and the picture preview.
                          Reporter identity and private notes stay with
                          handlers.
                        </p>
                        <label>
                          Household notice title
                          <input
                            value={title}
                            onChange={(event) => setTitle(event.target.value)}
                            required
                            minLength={5}
                            maxLength={120}
                          />
                        </label>
                        <label>
                          Household notice wording
                          <textarea
                            value={body}
                            onChange={(event) => setBody(event.target.value)}
                            required
                            minLength={10}
                            maxLength={4000}
                            rows={5}
                          />
                        </label>
                        <label>
                          Response by
                          <input
                            type="date"
                            value={responseBy}
                            onChange={(event) =>
                              setResponseBy(event.target.value)
                            }
                            required
                            min="1900-01-01"
                            max="2100-12-31"
                          />
                        </label>
                        <IncidentReviewFacts>
                          <span>
                            Recipient home<strong>{x.home}</strong>
                          </span>
                          <span>
                            Frozen supporting picture
                            <strong>
                              {x.picture_id ? 'Preview included' : 'No picture'}
                            </strong>
                          </span>
                        </IncidentReviewFacts>
                      </>
                    )}
                    <label>
                      {action === 'NOTE'
                        ? 'Private staff note'
                        : 'Reason for this decision'}
                      <textarea
                        value={reason}
                        onChange={(event) => setReason(event.target.value)}
                        required
                        minLength={10}
                        maxLength={2000}
                        rows={4}
                      />
                    </label>
                    <IncidentCheck checked={checked} onChange={setChecked}>
                      I reviewed this decision and its intended audience.
                    </IncidentCheck>
                  </fieldset>
                  <IncidentFeedback writer={writer} onReload={reload} />
                  <button
                    className="button button-dark"
                    disabled={
                      writer.busy ||
                      !action ||
                      !checked ||
                      (action === 'DUPLICATE' &&
                        (duplicates.loading || !!duplicates.error))
                    }
                  >
                    {writer.busy
                      ? 'Saving decision…'
                      : writer.locked
                        ? 'Retry incident decision'
                        : action === 'ISSUE_NOTICE'
                          ? 'Issue reviewed response notice'
                          : 'Save incident decision'}
                    <Icon name="check" />
                  </button>
                </form>
              )}
              {!x.can_participate && (
                <p className="form-help">
                  Your former society relationship has ended. This personal
                  record is read only.
                </p>
              )}
              <IncidentHistory
                events={x.events}
                total={x.event_total}
                page={x.event_page}
                onPage={setPage}
                disabled={writer.busy || writer.locked}
              />
            </>
          )}
        </IncidentUnavailable>
      </div>
    </PortalDialog>
  )
}
export function IncidentNoticeDialog({
  id,
  onClose,
  onSaved
}: {
  id: string
  onClose: () => void
  onSaved: () => void
}) {
  const [page, setPage] = useState(1),
    load = useIncidentLoad<IncidentNotice>(
      '/api/incident-notices/' +
        encodeURIComponent(id) +
        '?response_page=' +
        page
    ),
    writer = useIncidentWrite(),
    [body, setBody] = useState(''),
    [checked, setChecked] = useState(false),
    x = load.data
  const reload = () => {
      writer.reset()
      load.reload()
    },
    save = async (event: FormEvent) => {
      event.preventDefault()
      if (!x || !checked) return
      const result = await writer.send(
        '/api/incident-notices/' + encodeURIComponent(id) + '/responses',
        { version: x.version, body }
      )
      if (result) {
        setBody('')
        setChecked(false)
        load.reload()
        onSaved()
      }
    }
  return (
    <PortalDialog
      titleId="response-title"
      closeLabel="Close response notice"
      className="conduct-dialog"
      onClose={onClose}
      busy={writer.busy}
    >
      <div className="dialog-scroll detail-body incident-dialog">
        <span className="eyebrow">A CHANCE TO BE HEARD</span>
        <h2 id="response-title">
          {load.error
            ? 'Notice unavailable'
            : load.loading
              ? 'Opening this notice…'
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
                <span className="fund-state">
                  {x.active ? 'Current response notice' : 'Removed notice'}
                </span>
                <button
                  type="button"
                  className="text-link"
                  disabled={writer.busy || writer.locked}
                  onClick={load.reload}
                >
                  Refresh notice
                  <Icon name="refresh" />
                </button>
              </div>
              <IncidentReviewFacts>
                <span>
                  Your home<strong>{x.home}</strong>
                </span>
                <span>
                  Incident date<strong>{displayDate(x.incident_date)}</strong>
                </span>
                <span>
                  Response by<strong>{displayDate(x.response_by)}</strong>
                </span>
              </IncidentReviewFacts>
              <p className="notice-body preserve-lines">{x.body}</p>
              <RuleText rule={x.rule} />
              {x.picture_id && <IncidentPicture id={x.picture_id} />}
              <p className="incident-no-charge">
                <Icon name="shield" />A request for your response creates no
                charge.
              </p>
              <section
                className="review-history"
                aria-label="Household responses"
              >
                <h3>The responses, retained.</h3>
                {!x.response_total ? (
                  <p>No response has been submitted yet.</p>
                ) : (
                  <ol>
                    {x.responses.map((response) => (
                      <li key={response.id}>
                        <p>{response.body}</p>
                        <small>
                          {response.actor ? response.actor + ' · ' : ''}
                          {careTime(response.created_at)}
                        </small>
                      </li>
                    ))}
                  </ol>
                )}
                {x.response_total > 20 && (
                  <PageControls
                    label="household responses"
                    page={x.response_page}
                    total={x.response_total}
                    size={20}
                    onPage={setPage}
                    disabled={writer.busy || writer.locked}
                  />
                )}
              </section>
              {x.can_respond ? (
                <form className="portal-form incident-decision" onSubmit={save}>
                  <h3>Your account matters.</h3>
                  <fieldset
                    className="records-fieldset"
                    disabled={writer.busy || writer.locked}
                  >
                    <label>
                      Your household response
                      <textarea
                        value={body}
                        onChange={(event) => setBody(event.target.value)}
                        required
                        minLength={10}
                        maxLength={4000}
                        rows={5}
                      />
                    </label>
                    <IncidentCheck checked={checked} onChange={setChecked}>
                      I reviewed my response to this notice.
                    </IncidentCheck>
                  </fieldset>
                  <p className="form-help">
                    Your response is visible to you and authorised handlers.
                    Other residents do not receive your response.
                  </p>
                  <IncidentFeedback writer={writer} onReload={reload} />
                  <button
                    className="button button-dark"
                    disabled={writer.busy || !checked}
                  >
                    {writer.busy
                      ? 'Submitting response…'
                      : writer.locked
                        ? 'Retry this response'
                        : 'Submit household response'}
                    <Icon name="arrow" />
                  </button>
                </form>
              ) : (
                <p className="form-help">
                  {x.active
                    ? 'This is the handling team’s retained copy. Responses require a current relationship to the tagged home.'
                    : 'This notice was deliberately removed. Its original and responses remain in the handling record.'}
                </p>
              )}
            </>
          )}
        </IncidentUnavailable>
      </div>
    </PortalDialog>
  )
}
