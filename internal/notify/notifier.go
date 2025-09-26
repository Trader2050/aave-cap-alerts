package notify

import "context"

// Notifier delivers events to a downstream integration.
type Notifier interface {
	Notify(ctx context.Context, event SupplyChangeEvent) error
}
