package fakes

import "context"

// TxManager runs transactions over a Store one at a time. An error or a
// panic in fn puts every registered repository back to the state it had
// when the transaction began.
type TxManager struct {
	store *Store
}

func NewTxManager(s *Store) *TxManager { return &TxManager{store: s} }

// WithinTx joins the transaction ctx already belongs to, if any; the outer
// call then decides about commit and rollback.
func (m *TxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	s := m.store
	if s.inTx(ctx) {
		return fn(ctx)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	restores := make([]func(), len(s.parts))
	for i, p := range s.parts {
		restores[i] = p.Snapshot()
	}
	committed := false
	defer func() {
		if committed {
			return
		}
		for _, restore := range restores {
			restore()
		}
	}()

	if err := fn(context.WithValue(ctx, txKey{}, s)); err != nil {
		return err
	}
	committed = true
	return nil
}
