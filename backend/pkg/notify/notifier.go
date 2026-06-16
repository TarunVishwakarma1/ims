package notify

import (
	"context"
	"fmt"
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
}

type notifier struct {
	notRepo  repository.NotificationRepository
	userRepo repository.UserRepository
	appURL   string // base url for action links, e.g. https://app.example.com
}

func NewNotifier(notRepo repository.NotificationRepository, userRepo repository.UserRepository, appURL string) Notifier {
	if appURL == "" {
		appURL = "http://localhost:3000"
	}
	return &notifier{notRepo: notRepo, userRepo: userRepo, appURL: appURL}
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
func (n *notifier) dispatch(ctx context.Context, to, subject, body string) {
	if to == "" {
		return
	}
	row := &domain.Notification{
		ID:          uuid.New(),
		Channel:     "email",
		Recipient:   to,
		Subject:     subject,
		BodyText:    body,
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
	body := fmt.Sprintf(
		"Your order has been received.\n\n"+
			"Order: %s\nTotal: %s\nStatus: %s\n\n"+
			"Track it here: %s/orders/%s\n",
		order.ID, money(order.TotalAmount), order.Status, n.appURL, order.ID)
	n.dispatch(ctx, to, subj, body)
}

func (n *notifier) OrderStatusChanged(ctx context.Context, order *domain.Order) {
	// Only certain transitions are interesting to the customer.
	switch order.Status {
	case domain.OrderStatusShipped,
		domain.OrderStatusDelivered,
		domain.OrderStatusCompleted,
		domain.OrderStatusReady,
		domain.OrderStatusAccepted:
		// fall through
	default:
		return
	}
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Order #%s is now %s", short(order.ID), order.Status)
	body := fmt.Sprintf(
		"Order %s status changed to: %s\n\n"+
			"View: %s/orders/%s\n",
		order.ID, order.Status, n.appURL, order.ID)
	n.dispatch(ctx, to, subj, body)
}

func (n *notifier) OrderCancelled(ctx context.Context, order *domain.Order, reason string) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Order #%s cancelled", short(order.ID))
	r := reason
	if r == "" {
		r = "no reason provided"
	}
	body := fmt.Sprintf(
		"Your order has been cancelled.\n\n"+
			"Order: %s\nReason: %s\n\n"+
			"If this was unexpected, contact support.\n\n"+
			"View: %s/orders/%s\n",
		order.ID, r, n.appURL, order.ID)
	n.dispatch(ctx, to, subj, body)
}

func (n *notifier) ReturnCreated(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Return request received for order #%s", short(order.ID))
	body := fmt.Sprintf(
		"Your return request has been recorded and is awaiting approval.\n\n"+
			"Return ID: %s\nOrder: %s\nExpected refund: %s\n\n"+
			"Track: %s/returns/%s\n",
		ret.ID, order.ID, money(ret.RefundAmount), n.appURL, ret.ID)
	n.dispatch(ctx, to, subj, body)
}

func (n *notifier) ReturnRefunded(ctx context.Context, ret *domain.ReturnRequest, order *domain.Order) {
	to := n.emailForUser(ctx, order.UserID, order.OrgID)
	subj := fmt.Sprintf("Refund issued for return #%s", short(ret.ID))
	body := fmt.Sprintf(
		"Your refund of %s has been issued. It typically takes 5–7 business days to appear.\n\n"+
			"Return: %s\nOrder: %s\n\n"+
			"View: %s/returns/%s\n",
		money(ret.RefundAmount), ret.ID, order.ID, n.appURL, ret.ID)
	n.dispatch(ctx, to, subj, body)
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
