package notify

import (
	"context"
	"encoding/json"
	"log/slog"

	webpush "github.com/SherClockHolmes/webpush-go"

	"prislage/internal/store"
)

type WebPush struct {
	Items      *store.Items
	PublicKey  string
	PrivateKey string
	Subject    string
	Log        *slog.Logger
}

func (w *WebPush) Enabled() bool {
	return w != nil && w.PublicKey != "" && w.PrivateKey != ""
}

func (w *WebPush) Send(ctx context.Context, title, body string) error {
	if !w.Enabled() {
		return nil
	}
	subs, err := w.Items.ListPushSubscriptions(ctx)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"title": title, "body": body})
	for _, sub := range subs {
		s := &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256dh,
				Auth:   sub.Auth,
			},
		}
		resp, err := webpush.SendNotificationWithContext(ctx, payload, s, &webpush.Options{
			Subscriber:      w.Subject,
			VAPIDPublicKey:  w.PublicKey,
			VAPIDPrivateKey: w.PrivateKey,
			TTL:             60,
		})
		if err != nil {
			if w.Log != nil {
				w.Log.Warn("webpush failed", "endpoint", sub.Endpoint, "err", err)
			}
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == 404 || resp.StatusCode == 410 {
			_ = w.Items.DeletePushSubscription(ctx, sub.Endpoint)
		}
	}
	return nil
}
