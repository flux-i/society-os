import { useEffect, useRef, useState } from 'react'
import type { FormEvent } from 'react'
import { localToday, useIncidentLoad, useIncidentWrite } from '../incidents'
import type {
  IncidentDetail,
  IncidentOptions,
  Rule,
  RulePage
} from '../incidents'
import { displayDate } from '../maintenance'
import { FormSelect } from './FilterSelect'
import { PortalDialog } from './PortalDialog'
import { PageControls } from './Maintenance'
import {
  IncidentCheck,
  IncidentFeedback,
  IncidentPicture,
  IncidentReviewFacts,
  PictureUpload
} from './IncidentShared'
import { RuleText } from './IncidentRules'
import { Icon } from './Icon'
export function IncidentReportForm({
  existing,
  onClose,
  onSaved
}: {
  existing?: IncidentDetail
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const [home, setHome] = useState(existing?.flat_id ?? ''),
    [homeQuery, setHomeQuery] = useState(''),
    [selectedHome, setSelectedHome] = useState(existing?.home ?? ''),
    [rule, setRule] = useState<Rule | null>(existing?.rule ?? null),
    [ruleQuery, setRuleQuery] = useState(''),
    [rulePage, setRulePage] = useState(1),
    [when, setWhen] = useState(existing?.incident_date ?? localToday()),
    [comment, setComment] = useState(existing?.comment ?? ''),
    [picture, setPicture] = useState(existing?.picture_id ?? ''),
    [picturePending, setPicturePending] = useState(false),
    [review, setReview] = useState(false),
    [checked, setChecked] = useState(false),
    writer = useIncidentWrite(),
    heading = useRef<HTMLHeadingElement>(null)
  const homes = useIncidentLoad<IncidentOptions>(
      '/api/incidents/options?' + new URLSearchParams({ q: homeQuery })
    ),
    rules = useIncidentLoad<RulePage>(
      '/api/rules?' +
        new URLSearchParams({
          q: ruleQuery,
          state: 'PUBLISHED',
          page: String(rulePage)
        })
    ),
    blocked =
      homes.loading ||
      !!homes.error ||
      rules.loading ||
      !!rules.error ||
      picturePending
  useEffect(() => {
    if (review) heading.current?.focus()
  }, [review])
  const homeOptions = homes.data?.homes ?? [],
    ruleOptions = rules.data?.items ?? []
  const selectedHomes =
      home && !homeOptions.some((value) => value.id === home)
        ? [{ id: home, label: selectedHome }, ...homeOptions]
        : homeOptions,
    selectedRules =
      rule && !ruleOptions.some((value) => value.id === rule.id)
        ? [rule, ...ruleOptions]
        : ruleOptions
  const save = async (event: FormEvent) => {
    event.preventDefault()
    if (!rule || !checked || picturePending) return
    const result = await writer.send(
      existing
        ? '/api/incidents/' + encodeURIComponent(existing.id) + '/revision'
        : '/api/incidents',
      {
        version: existing?.version ?? 0,
        rule_id: rule.id,
        flat_id: home,
        incident_date: when,
        comment,
        picture_id: picture
      }
    )
    if (result) onSaved(result.id)
  }
  return (
    <PortalDialog
      titleId="report-title"
      closeLabel="Close incident report"
      className="conduct-dialog"
      onClose={onClose}
      busy={writer.busy || picturePending}
    >
      <div className="dialog-scroll detail-body incident-dialog">
        <span className="eyebrow">A PRIVATE OBSERVATION</span>
        <h2 id="report-title" ref={heading} tabIndex={-1}>
          {review ? (
            'Review your report.'
          ) : existing ? (
            <>
              Keep the facts
              <br />
              <em>together.</em>
            </>
          ) : (
            <>
              Something needs
              <br />
              <em>a closer look.</em>
            </>
          )}
        </h2>
        <p className="detail-intro">
          Share the observation with the handling team. A separate review
          decides what happens next.
        </p>
        {review ? (
          <form className="portal-form" onSubmit={save}>
            <IncidentReviewFacts>
              <span>
                Tagged home<strong>{selectedHome}</strong>
              </span>
              <span>
                Incident date<strong>{displayDate(when)}</strong>
              </span>
            </IncidentReviewFacts>
            {rule && <RuleText rule={rule} />}
            <h3>Your observation</h3>
            <p className="preserve-lines">{comment}</p>
            {picture && <IncidentPicture id={picture} original />}
            <p className="form-help">
              Only you and authorised handlers can read this report. A household
              response requires a separate, deliberately issued notice.
            </p>
            <IncidentCheck
              checked={checked}
              onChange={setChecked}
              disabled={writer.busy || writer.locked}
            >
              I reviewed the home, rule, date and supporting facts.
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
                Edit report
              </button>
              <button
                className="button button-dark"
                disabled={writer.busy || !checked}
              >
                {writer.busy
                  ? 'Submitting report…'
                  : writer.locked
                    ? 'Retry this report'
                    : existing
                      ? 'Submit revised report'
                      : 'Submit private report'}
                <Icon name="arrow" />
              </button>
            </div>
          </form>
        ) : (
          <form
            className="portal-form"
            onSubmit={(event) => {
              event.preventDefault()
              if (rule && !blocked) setReview(true)
            }}
          >
            <fieldset className="records-fieldset" disabled={picturePending}>
              <label>
                Find a home
                <input
                  type="search"
                  value={homeQuery}
                  onChange={(event) => setHomeQuery(event.target.value)}
                  maxLength={100}
                  placeholder="Wing and home number…"
                />
              </label>
              {homes.error ? (
                <div className="form-error" role="alert">
                  <p>{homes.error}</p>
                  <button
                    type="button"
                    className="text-link"
                    onClick={homes.reload}
                  >
                    Retry home choices
                    <Icon name="refresh" />
                  </button>
                </div>
              ) : (
                <label>
                  Tagged home
                  <FormSelect
                    label="Tagged home"
                    value={home}
                    onChange={(value) => {
                      setHome(value)
                      setSelectedHome(
                        selectedHomes.find((item) => item.id === value)
                          ?.label ?? ''
                      )
                    }}
                    disabled={homes.loading}
                    required
                    options={[
                      {
                        value: '',
                        label: homes.loading
                          ? 'Opening homes…'
                          : 'Choose the observed home…'
                      },
                      ...selectedHomes.map((item) => ({
                        value: item.id,
                        label: 'Home ' + item.label
                      }))
                    ]}
                  />
                </label>
              )}
              <p className="form-help">
                You can tag another flat. This selector contains home numbers;
                residents’ personal details stay private.
              </p>
              <label>
                Find a rule
                <input
                  type="search"
                  value={ruleQuery}
                  onChange={(event) => {
                    setRuleQuery(event.target.value)
                    setRulePage(1)
                  }}
                  maxLength={100}
                  placeholder="A published policy title…"
                />
              </label>
              {rules.error ? (
                <div className="form-error" role="alert">
                  <p>{rules.error}</p>
                  <button
                    type="button"
                    className="text-link"
                    onClick={rules.reload}
                  >
                    Retry rule choices
                    <Icon name="refresh" />
                  </button>
                </div>
              ) : (
                <label>
                  Applicable published rule
                  <FormSelect
                    label="Applicable published rule"
                    value={rule?.id ?? ''}
                    onChange={(value) =>
                      setRule(
                        selectedRules.find((item) => item.id === value) ?? null
                      )
                    }
                    disabled={rules.loading}
                    required
                    options={[
                      {
                        value: '',
                        label: rules.loading
                          ? 'Opening rules…'
                          : rules.data?.total
                            ? 'Choose the applicable rule…'
                            : 'No published rules yet'
                      },
                      ...selectedRules.map((item) => ({
                        value: item.id,
                        label: item.title
                      }))
                    ]}
                  />
                </label>
              )}
              {rules.data && rules.data.total > 12 && (
                <PageControls
                  label="rule choices"
                  page={rules.data.page}
                  total={rules.data.total}
                  size={12}
                  onPage={setRulePage}
                  disabled={rules.loading}
                />
              )}
              <label>
                Incident date
                <input
                  type="date"
                  value={when}
                  onChange={(event) => setWhen(event.target.value)}
                  required
                  min="1900-01-01"
                  max={localToday()}
                />
              </label>
              {rule && <RuleText rule={rule} />}{' '}
              {rule?.state === 'RETIRED' && (
                <p className="form-error" role="alert">
                  The original rule is retired. Choose a currently published
                  applicable rule before resubmitting.
                </p>
              )}
              <label>
                Your observation
                <textarea
                  value={comment}
                  onChange={(event) => setComment(event.target.value)}
                  minLength={10}
                  maxLength={4000}
                  required
                  rows={5}
                  placeholder="What did you observe? Include the facts that would help a fair review."
                />
              </label>
            </fieldset>
            <PictureUpload
              value={picture}
              onChange={setPicture}
              onBusy={setPicturePending}
            />
            {existing?.notice_id && (
              <p className="form-help">
                A household response notice is active. A handler must remove and
                reissue it before the tagged facts or picture can change.
              </p>
            )}
            <button
              className="button button-dark"
              disabled={blocked || !home || !rule || rule.state !== 'PUBLISHED'}
            >
              Review private report
              <Icon name="arrow" />
            </button>
          </form>
        )}
      </div>
    </PortalDialog>
  )
}
