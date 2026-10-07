package messaging

import (
	"context"
	"time"

	"society.local/portal/internal/database"
)

func (e *Engine) Configuration() map[string]any {
	mode := "SIMULATION"
	templates := map[string]bool{}
	if e.whatsapp != nil {
		mode = e.whatsapp.mode
		for kind := range e.whatsapp.config.Templates {
			templates[kind] = true
		}
	}
	return map[string]any{"simulation_enabled": true, "whatsapp_live": false, "email_live": false, "whatsapp_provider_mode": mode, "whatsapp_configured_sources": templates, "attempt_limit": 3, "dispatch_limit": 25}
}

func (e *Engine) Preview(ctx context.Context, token string, in database.MessageInput, page int) (database.MessagePreview, error) {
	if e.whatsapp != nil && in.Channel == "WHATSAPP" {
		if _, err := e.Store.CheckMessageWrite(ctx, token, "", "PREVIEW", in, in.SourceKind); err != nil {
			return database.MessagePreview{}, err
		}
		provider, err := e.whatsapp.ReadTemplate(ctx, in.SourceKind)
		if err != nil {
			return database.MessagePreview{}, err
		}
		in.Provider = provider
	}
	return e.Store.MessagePreviewFor(ctx, token, in, page)
}

func (e *Engine) Propose(ctx context.Context, token string, in database.MessageInput) (string, error) {
	if e.whatsapp != nil && in.Channel == "WHATSAPP" {
		id, err := e.Store.CheckMessageWrite(ctx, token, "", "MESSAGE_PROPOSE", in, in.SourceKind)
		if err != nil || id != "" {
			return id, err
		}
		provider, err := e.whatsapp.ReadTemplate(ctx, in.SourceKind)
		if err != nil {
			return "", err
		}
		in.Provider = provider
	}
	return e.Store.ProposeMessage(ctx, token, in)
}

func (e *Engine) Act(ctx context.Context, token, id string, in database.MessageAction) (string, error) {
	x, err := e.Store.MessageFor(ctx, token, id, 1, 1, 1)
	if err != nil {
		return "", err
	}
	if x.Provider != nil && (in.Action == "APPROVED" || in.Action == "REFRESH") {
		result, err := e.Store.CheckMessageWrite(ctx, token, id, "MESSAGE_ACTION:"+id, in, "")
		if err != nil || result != "" {
			return result, err
		}
		if e.whatsapp == nil {
			return "", database.InvalidMessageProvider("The configured WhatsApp provider is unavailable.")
		}
		provider, err := e.whatsapp.ReadTemplate(ctx, x.Source.Kind)
		if err != nil {
			return "", err
		}
		in.Provider = provider
	}
	return e.Store.ActOnMessage(ctx, token, id, in)
}

func (e *Engine) dispatchWhatsApp(ctx context.Context, token, id string, in database.MessageAction) (string, error) {
	result, err := e.Store.CheckMessageWrite(ctx, token, id, "MESSAGE_DISPATCH:"+id, in, "")
	if err != nil || result != "" {
		return result, err
	}
	if e.whatsapp == nil {
		return "", database.InvalidMessageProvider("The configured WhatsApp provider is unavailable.")
	}
	x, err := e.Store.MessageFor(ctx, token, id, 1, 1, 1)
	if err != nil {
		return "", err
	}
	provider, err := e.whatsapp.ReadTemplate(ctx, x.Source.Kind)
	if err != nil {
		return "", err
	}
	if !database.SameMessageProvider(provider, x.Provider) {
		return "", database.ErrConflict
	}
	in.Provider = provider
	claims, result, err := e.Store.ClaimMessageDispatch(ctx, token, id, in)
	if err != nil {
		return "", err
	}
	// Every claim is safely abandoned if template checks, context cancellation,
	// database writes or the process interrupt the remaining handoffs.
	defer func() {
		recovery, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = e.Store.AbandonMessageClaims(recovery, claims)
	}()
	for _, claim := range claims {
		current, err := e.whatsapp.ReadTemplate(ctx, x.Source.Kind)
		if err != nil {
			return "", err
		}
		if !database.SameMessageProvider(current, x.Provider) {
			return "", database.ErrConflict
		}
		attempt, err := e.Store.PrepareWhatsAppHandoff(ctx, token, claim.ID, current)
		if err != nil {
			return "", err
		}
		if attempt.ID == "" {
			continue
		}
		response := e.whatsapp.Send(ctx, attempt)
		// Persist even when the initiating browser loses its request context.
		finish, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		err = e.Store.CompleteWhatsAppHandoff(finish, claim.ID, response)
		cancel()
		if err != nil {
			return "", err
		}
	}
	return result, nil
}

func (e *Engine) WhatsAppChallenge(query map[string][]string) (string, error) {
	if e.whatsapp == nil {
		return "", database.ErrForbidden
	}
	return e.whatsapp.VerifyChallenge(query)
}

func (e *Engine) WhatsAppCallback(ctx context.Context, payload []byte, signature string, now time.Time) error {
	if e.whatsapp == nil {
		return database.ErrForbidden
	}
	statuses, err := e.whatsapp.SignedStatuses(payload, signature, now)
	if err != nil {
		return err
	}
	return e.Store.ReceiveWhatsAppStatuses(ctx, statuses)
}
