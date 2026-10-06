import { useEffect, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { APIError, uploadIncidentPicture } from '../api'
import { incidentActions, useIncidentWrite } from '../incidents'
import type { Picture } from '../incidents'
import type { FundEvent } from '../collections'
import { careTime } from '../upkeep'
import { PageControls } from './Maintenance'
import { Icon } from './Icon'
import { FundFeedback } from './FundShared'
export {
  FundCheck as IncidentCheck,
  FundUnavailable as IncidentUnavailable
} from './FundShared'
export function IncidentFeedback({
  writer,
  onReload
}: {
  writer: ReturnType<typeof useIncidentWrite>
  onReload?: () => void
}) {
  useEffect(() => {
    if (!writer.error) return
    const frame = requestAnimationFrame(() => {
      const button = writer.feedback.current
        ?.closest('form')
        ?.querySelector<HTMLButtonElement>('.button-dark')
      button?.scrollIntoView({ block: 'nearest', behavior: 'instant' })
    })
    return () => cancelAnimationFrame(frame)
  }, [writer.error])
  return <FundFeedback writer={writer} onReload={onReload} />
}
export function IncidentHistory({
  events,
  total,
  page,
  onPage,
  disabled = false
}: {
  events: FundEvent[]
  total: number
  page: number
  onPage: (value: number) => void
  disabled?: boolean
}) {
  return (
    <section
      className="review-history upkeep-history"
      aria-label="Incident activity"
    >
      <h3>A clear record of the decisions.</h3>
      <ol>
        {events.map((event) => (
          <li key={event.version}>
            <strong>{incidentActions[event.action] ?? event.action}</strong>
            <p>{event.reason}</p>
            <small>
              {event.actor} · {careTime(event.at)}
            </small>
          </li>
        ))}
      </ol>
      {total > 20 && (
        <PageControls
          label="incident activity"
          page={page}
          total={total}
          size={20}
          onPage={onPage}
          disabled={disabled}
        />
      )}
    </section>
  )
}
export function IncidentPicture({
  id,
  original = false
}: {
  id: string
  original?: boolean
}) {
  const [error, setError] = useState(false),
    [revision, setRevision] = useState(0)
  useEffect(() => {
    setError(false)
  }, [id, revision])
  return (
    <section className="incident-picture" aria-label="Supporting picture">
      {error ? (
        <div className="form-error" role="alert">
          <p>The picture could not be opened.</p>
          <button
            type="button"
            className="text-link"
            onClick={() => setRevision((value) => value + 1)}
          >
            Retry picture
            <Icon name="refresh" />
          </button>
        </div>
      ) : (
        <img
          src={
            '/api/incident-pictures/' +
            encodeURIComponent(id) +
            '/preview?retry=' +
            revision
          }
          onError={() => setError(true)}
          alt="Validated picture supplied for this case"
        />
      )}
      <div className="incident-picture-footer">
        <p>
          {original
            ? 'Private original retained. The preview omits device and location metadata.'
            : 'This preview was deliberately shared for your response.'}
        </p>
        {original && (
          <a
            className="text-link"
            href={
              '/api/incident-pictures/' + encodeURIComponent(id) + '/original'
            }
            download
          >
            Download original
            <Icon name="document" />
          </a>
        )}
      </div>
    </section>
  )
}
export function PictureUpload({
  value,
  onChange,
  onBusy,
  disabled = false
}: {
  value: string
  onChange: (id: string) => void
  onBusy: (value: boolean) => void
  disabled?: boolean
}) {
  const [busy, setBusy] = useState(false),
    [error, setError] = useState(''),
    [locked, setLocked] = useState(false),
    [name, setName] = useState(''),
    pending = useRef<{
      file: File
      key: string
    } | null>(null),
    inFlight = useRef(false)
  const send = async () => {
    if (!pending.current || inFlight.current) return
    inFlight.current = true
    setBusy(true)
    onBusy(true)
    setError('')
    let unresolved = false
    try {
      const x = await uploadIncidentPicture<Picture>(
        pending.current.file,
        pending.current.key
      )
      onChange(x.id)
      setName(x.filename)
      pending.current = null
      setLocked(false)
    } catch (err) {
      setError((err as Error).message)
      const rejected =
        err instanceof APIError && [400, 403, 404].includes(err.status)
      setLocked(!rejected)
      unresolved = !rejected
      if (rejected) pending.current = null
    } finally {
      inFlight.current = false
      setBusy(false)
      onBusy(unresolved)
    }
  }
  return (
    <section className="picture-upload">
      <label>
        Supporting picture (optional)
        <input
          aria-label="Supporting picture (optional)"
          type="file"
          accept="image/png,image/jpeg"
          capture="environment"
          disabled={disabled || busy || locked}
          onChange={(event) => {
            const file = event.target.files?.[0]
            if (!file) return
            pending.current = { file, key: crypto.randomUUID() }
            setName(file.name)
            void send()
          }}
        />
      </label>
      <p className="form-help">
        Use your camera or choose a PNG/JPEG, up to 5 MiB and eight million
        pixels. This stays with your private report.
      </p>
      {busy && <p role="status">Checking your picture…</p>}
      {error && (
        <div className="form-error" role="alert">
          <p>{error}</p>
          {locked && (
            <div className="incident-action-tools">
              <button
                type="button"
                className="text-link"
                disabled={busy}
                onClick={() => void send()}
              >
                Retry this picture
                <Icon name="refresh" />
              </button>
              <button
                type="button"
                className="text-link"
                disabled={busy}
                onClick={() => {
                  pending.current = null
                  setLocked(false)
                  setError('')
                  setName('')
                  onBusy(false)
                }}
              >
                Choose another picture
                <Icon name="arrow" />
              </button>
            </div>
          )}
        </div>
      )}
      {name && !busy && !error && (
        <p className="form-success" role="status">
          {name} · private picture ready
        </p>
      )}
      {value && !busy && !error && (
        <>
          <IncidentPicture id={value} />
          <button
            type="button"
            className="text-link"
            disabled={disabled}
            onClick={() => {
              onChange('')
              setName('')
            }}
          >
            Leave picture out
            <Icon name="close" />
          </button>
        </>
      )}
    </section>
  )
}
export function IncidentReviewFacts({ children }: { children: ReactNode }) {
  return <div className="incident-review-facts">{children}</div>
}
