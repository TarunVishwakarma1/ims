package notify

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/TarunVishwakarma1/ims/backend/internal/domain"
	"github.com/TarunVishwakarma1/ims/backend/internal/repository"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Notifier is the high-level interface services call to send transactional
// notifications. Implementations look up the recipient (via UserRepository),
// render a message, and dispatch through an Emailer. All methods are
// best-effort — failures are logged, never returned, so the calling
// business flow doesn't get tied to email delivery.
type Notifier interface {
	OrderCreated(ctx context.Context, order *domain.Order)
	OrderStatusChanged(ctx context.Context, order *domain.Order)
	OrderCancelled(ctx context.Context, order *domain.Order, reason string)
	ReturnCreated(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order)
	ReturnRefunded(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order)

	// EmailVerificationOTP sends the OTP used to verify a newly-created
	// email address. Subject + body use the OTP template.
	EmailVerificationOTP(ctx context.Context, email, otp string, ttlMinutes int)

	// PasswordReset sends the password-reset link with the raw token
	// embedded in the URL. Subject + body use the password_reset template.
	PasswordReset(ctx context.Context, email, rawToken string, ttlMinutes int)
}

type notifier struct {
	notRepo  repository.NotificationRepository
	userRepo repository.UserRepository
	renderer *Renderer
	appURL   string // base url for action links, e.g. https://app.example.com
}

func NewNotifier(notRepo repository.NotificationRepository, userRepo repository.UserRepository, appURL string) Notifier {
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	// Template rendering. If the templates failed to parse at boot we still
	// return a working notifier with renderer=nil; dispatch falls back to
	// plain text so nothing silently disappears.
	rend, err := NewRenderer()
	if err != nil {
		zap.L().Error("notify template parse failed — falling back to plain text", zap.Error(err))
	}
	return &notifier{notRepo: notRepo, userRepo: userRepo, renderer: rend, appURL: appURL}
}

// dispatch persists the message to the notifications table so the worker
// can pick it up. Errors here are logged but never propagated — failing
// to enqueue an email must not fail the business operation that triggered
// it. Background worker (jobs.StartNotificationWorker) drains the queue.
//
// CONTEXT HANDLING: the caller's request ctx is INTENTIONALLY NOT used as
// the parent of the DB call. When an HTTP handler returns, its ctx is
// cancelled; the enqueue would then fail with context.Canceled and the
// user would never get their email. Instead we derive a fresh background
// ctx with a tight deadline. Trace IDs from caller ctx (if any) are
// propagated via TraceIDFromContext below.
// dispatchHTML renders a templated email and enqueues both the HTML body
// and the plain-text fallback. Callers pass a template name (matches the
// HTML filename without extension) and a data struct that satisfies that
// template's expectations.
func (n *notifier) dispatchHTML(ctx context.Context, to, subject, template string, data any, fallbackText string) {
	if to == "" {
		return
	}
	html, text := "", fallbackText
	if n.renderer != nil {
		h, t, err := n.renderer.render(template, data)
		if err != nil {
			zap.L().Warn("notify render failed — sending text only",
				zap.String("template", template), zap.Error(err))
		} else {
			html = h
			text = t
		}
	}
	n.dispatch(ctx, to, subject, text, html)
}

func (n *notifier) dispatch(ctx context.Context, to, subject, body, html string) {
	if to == "" {
		return
	}
	var htmlPtr *string
	if html != "" {
		htmlPtr = &html
	}
	row := &domain.Notification{
		ID:          uuid.New(),
		Channel:     "email",
		Recipient:   to,
		Subject:     subject,
		BodyText:    body,
		BodyHTML:    htmlPtr,
		Status:      domain.NotificationStatusPending,
		NextAttempt: time.Now().UTC(),
		CreatedAt:   time.Now().UTC(),
	}
	enqCtx, cancel := context.WithTimeout(detachContext(ctx), 2*time.Second)
	defer cancel()
	if err := n.notRepo.Enqueue(enqCtx, row); err != nil {
		zap.L().Warn("notification enqueue failed",
			zap.String("to", to),
			zap.String("subject", subject),
			zap.Error(err))
	}
}

// detachContext returns a fresh context that inherits values from `parent`
// (so future tracing / log-correlation IDs survive) but NOT its
// cancellation. Used wherever we want logical decoupling from request
// lifetime without losing observability metadata.
func detachContext(parent context.Context) context.Context {
	return detachedContext{Context: context.Background(), parent: parent}
}

type detachedContext struct {
	context.Context
	parent context.Context
}

func (d detachedContext) Value(key any) any { return d.parent.Value(key) }

// emailForUser looks up the recipient. Returns empty string on lookup
// failure — caller's dispatch() handles that by skipping the send.
func (n *notifier) emailForUser(ctx context.Context, userID, orgID uuid.UUID) string {
	u, err := n.userRepo.GetByID(ctx, userID, orgID)
	if err != nil {
		zap.L().Warn("notify: recipient lookup failed",
			zap.String("user_id", userID.String()), zap.Error(err))
		return ""
	}
	return u.Email
}

func (n *notifier) OrderCreated(ctx context.Context, order *domain.Order) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Order #%s confirmed", short(order.ID))
	data := struct {
		baseData
		Hero   hero
		Amount amount
		Rows   []kv
		CTA    cta
	}{
		baseData: newBase(subj, "ORDER"),
		Hero:     hero{Title: "Order confirmed", Subtitle: "Thanks for your order — we've got it from here."},
		Amount:   amount{Label: "Order total", Amount: money(order.TotalAmount), Sub: "Including taxes"},
		Rows: []kv{
			{"Order ID", "#" + short(order.ID)},
			{"Status", strings.Title(string(order.Status))},
			{"Placed", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		CTA: cta{URL: n.appURL + "/orders/" + order.ID.String(), Label: "View order"},
	}
	fallback := fmt.Sprintf("Order #%s confirmed. Total: %s. Track: %s/orders/%s",
		short(order.ID), money(order.TotalAmount), n.appURL, order.ID)
	n.dispatchHTML(ctx, to, subj, "order_placed", data, fallback)
}

func (n *notifier) OrderStatusChanged(ctx context.Context, order *domain.Order) {
	// Only certain transitions are interesting to the customer.
	switch order.Status {
	case domain.OrderStatusShipped,
		domain.OrderStatusDelivered,
		domain.OrderStatusCompleted,
		domain.OrderStatusReady,
		domain.OrderStatusAccepted:
	default:
		return
	}
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	titleMap := map[domain.OrderStatus]string{
		domain.OrderStatusAccepted:  "Order accepted",
		domain.OrderStatusReady:     "Order ready for shipment",
		domain.OrderStatusShipped:   "Your order is on the way",
		domain.OrderStatusDelivered: "Order delivered",
		domain.OrderStatusCompleted: "Order completed",
	}
	subj := fmt.Sprintf("%s — Order #%s", titleMap[order.Status], short(order.ID))
	data := struct {
		baseData
		Hero  hero
		Alert alert
		Rows  []kv
		CTA   cta
	}{
		baseData: newBase(subj, strings.ToUpper(string(order.Status))),
		Hero:     hero{Title: titleMap[order.Status], Subtitle: "Status update for your order."},
		Alert:    alert{Title: "Status updated", Body: "Your order is now " + string(order.Status) + "."},
		Rows: []kv{
			{"Order ID", "#" + short(order.ID)},
			{"New status", strings.Title(string(order.Status))},
			{"Updated", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		CTA: cta{URL: n.appURL + "/orders/" + order.ID.String(), Label: "Track order"},
	}
	fallback := fmt.Sprintf("Order #%s is now %s. View: %s/orders/%s",
		short(order.ID), order.Status, n.appURL, order.ID)
	n.dispatchHTML(ctx, to, subj, "order_status", data, fallback)
}

func (n *notifier) OrderCancelled(ctx context.Context, order *domain.Order, reason string) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	r := reason
	if r == "" {
		r = "no reason provided"
	}
	subj := fmt.Sprintf("Order #%s cancelled", short(order.ID))
	type data struct {
		baseData
		Hero   hero
		Rows   []kv
		Refund *amount
		Alert  *alert
		CTA    cta
	}
	d := data{
		baseData: newBase(subj, "CANCELLED"),
		Hero:     hero{Title: "Order cancelled", Subtitle: "Reserved stock has been released."},
		Rows: []kv{
			{"Order ID", "#" + short(order.ID)},
			{"Reason", r},
			{"Cancelled", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		CTA: cta{URL: n.appURL + "/orders/" + order.ID.String(), Label: "Open order"},
	}
	if order.PaymentStatus == "paid" {
		d.Alert = &alert{Title: "Refund in progress", Body: "Per our cancellation policy, the refund will arrive in 5–7 business days."}
	}
	fallback := fmt.Sprintf("Order #%s cancelled. Reason: %s. View: %s/orders/%s",
		short(order.ID), r, n.appURL, order.ID)
	n.dispatchHTML(ctx, to, subj, "order_cancelled", d, fallback)
}

func (n *notifier) ReturnCreated(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Return request received — Order #%s", short(order.ID))
	data := struct {
		baseData
		Hero   hero
		Intro  string
		Amount *amount
		Rows   []kv
		CTA    cta
	}{
		baseData: newBase(subj, "RETURN"),
		Hero:     hero{Title: "Return requested", Subtitle: "Your request is awaiting approval from the supplier."},
		Intro:    "Once approved, we'll let you know how to send the items back.",
		Amount:   &amount{Label: "Expected refund", Amount: money(ret.RefundAmount), Sub: "On items received in good condition"},
		Rows: []kv{
			{"Return ID", "#" + short(ret.ID)},
			{"Order ID", "#" + short(order.ID)},
			{"Submitted", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		CTA: cta{URL: n.appURL + "/returns/" + ret.ID.String(), Label: "Track return"},
	}
	fallback := fmt.Sprintf("Return #%s requested. Expected refund: %s. Track: %s/returns/%s",
		short(ret.ID), money(ret.RefundAmount), n.appURL, ret.ID)
	n.dispatchHTML(ctx, to, subj, "return", data, fallback)
}

func (n *notifier) ReturnRefunded(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Refund issued — Return #%s", short(ret.ID))
	data := struct {
		baseData
		Hero   hero
		Amount amount
		Body   string
		Rows   []kv
		CTA    cta
	}{
		baseData: newBase(subj, "REFUND"),
		Hero:     hero{Title: "Refund issued", Subtitle: "Your refund is on the way."},
		Amount:   amount{Label: "Refund amount", Amount: money(ret.RefundAmount), Sub: "Back to your original payment method"},
		Body:     "We've processed your return refund. It should appear on your account in 5–7 business days.",
		Rows: []kv{
			{"Return ID", "#" + short(ret.ID)},
			{"Order ID", "#" + short(order.ID)},
			{"Issued", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		CTA: cta{URL: n.appURL + "/returns/" + ret.ID.String(), Label: "View return"},
	}
	fallback := fmt.Sprintf("Refund of %s issued for return #%s. View: %s/returns/%s",
		money(ret.RefundAmount), short(ret.ID), n.appURL, ret.ID)
	n.dispatchHTML(ctx, to, subj, "refund", data, fallback)
}

func (n *notifier) EmailVerificationOTP(ctx context.Context, email, otp string, ttlMinutes int) {
	if ttlMinutes <= 0 {
		ttlMinutes = 10
	}
	subj := "Verify your email — code: " + otp
	data := struct {
		baseData
		Hero   hero
		Action string
		Code   otpCode
		Alert  alert
	}{
		baseData: newBase(subj, "VERIFY"),
		Hero:     hero{Title: "Verify your email", Subtitle: "Confirm this address to finish setting up your IMS account."},
		Action:   "verify your email address",
		Code:     otpCode{Code: otp, Minutes: ttlMinutes},
		Alert:    alert{Title: "Keep it private", Body: "We will never ask for this code by phone or chat."},
	}
	fallback := fmt.Sprintf("Your IMS verification code is %s. Expires in %d minutes.", otp, ttlMinutes)
	n.dispatchHTML(ctx, email, subj, "otp", data, fallback)
}

func (n *notifier) PasswordReset(ctx context.Context, email, rawToken string, ttlMinutes int) {
	if ttlMinutes <= 0 {
		ttlMinutes = 60
	}
	resetURL := fmt.Sprintf("%s/reset-password?token=%s", n.appURL, rawToken)
	// Dev escape hatch: log the URL so the operator can copy it from the
	// backend log if the email lands in spam / fails to deliver.
	zap.L().Info("password reset URL issued",
		zap.String("to", email),
		zap.String("url", resetURL),
		zap.Int("ttl_min", ttlMinutes))
	subj := "Reset your IMS password"
	data := struct {
		baseData
		Hero  hero
		Body  string
		CTA   cta
		Rows  []kv
		Alert alert
	}{
		baseData: newBase(subj, "ACCOUNT"),
		Hero:     hero{Title: "Password reset", Subtitle: "Click the button below to set a new password."},
		Body:     "Someone requested a password reset for this account. If that wasn't you, ignore this email and your password stays the same.",
		CTA:      cta{URL: resetURL, Label: "Reset password"},
		Rows: []kv{
			{"Link expires in", fmt.Sprintf("%d minutes", ttlMinutes)},
			{"Requested at", time.Now().UTC().Format("2 Jan 2006, 15:04 UTC")},
		},
		Alert: alert{Title: "Security tip", Body: "We never ask for your password or the link via chat or phone."},
	}
	fallback := fmt.Sprintf("Reset your IMS password: %s (expires in %d minutes)", resetURL, ttlMinutes)
	n.dispatchHTML(ctx, email, subj, "password_reset", data, fallback)
}

// short returns the first uuid block — easier to read in subject lines.
func short(id uuid.UUID) string {
	s := id.String()
	if i := len(s); i >= 8 {
		return s[:8]
	}
	return s
}

// money formats paise → "₹123.45" without pulling in the i18n stack.
func money(paise int64) string {
	rupees := paise / 100
	rem := paise % 100
	if rem < 0 {
		rem = -rem
	}
	return fmt.Sprintf("₹%d.%02d", rupees, rem)
}
