package ports

import "context"

// UnitOfWork defines a transaction boundary abstraction.
// Implementations execute the provided function within a database transaction,
// committing on success and rolling back on failure. The context passed to fn
// carries the transaction scope for repository implementations to detect.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
