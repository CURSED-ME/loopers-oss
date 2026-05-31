package budget

import (
	"context"
	"time"

	"github.com/loopers-oss/loopers/internal/logging"
)

type reserveReq struct {
	estCost float64
	respCh  chan error
}

type reconcileReq struct {
	reservedCost float64
	actualCost   float64
}

// CheckAndReserve uses Micro-Batching to dramatically reduce Redis load.
// It groups concurrent requests for the same API key and performs a single
// atomic check against Redis every 5ms.
func (c *Client) CheckAndReserve(ctx context.Context, keyHash string, estCost float64) error {
	c.batchMu.Lock()
	if c.reserveBatches == nil {
		c.reserveBatches = make(map[string][]reserveReq)
		// Start flush timer if this is the first item in the batch
		time.AfterFunc(5*time.Millisecond, func() {
			c.flushReserveBatches()
		})
	}

	respCh := make(chan error, 1)
	c.reserveBatches[keyHash] = append(c.reserveBatches[keyHash], reserveReq{
		estCost: estCost,
		respCh:  respCh,
	})
	c.batchMu.Unlock()

	select {
	case err := <-respCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Client) flushReserveBatches() {
	c.batchMu.Lock()
	batches := c.reserveBatches
	c.reserveBatches = nil // reset for next batch
	c.batchMu.Unlock()

	for keyHash, reqs := range batches {
		go c.processReserveBatch(keyHash, reqs)
	}
}

func (c *Client) processReserveBatch(keyHash string, reqs []reserveReq) {
	// 1. Calculate total requested cost
	var totalCost float64
	for _, r := range reqs {
		totalCost += r.estCost
	}

	// 2. Fast Path: Attempt to reserve the entire batch in a single Redis roundtrip
	err := c.checkAndReserveRedis(context.Background(), keyHash, totalCost)

	// If successful, wake up all blocked requests!
	if err == nil {
		for _, r := range reqs {
			r.respCh <- nil
		}
		return
	}

	// 3. Strict Accuracy Fallback: If the batch exceeded the budget, we must process
	// them sequentially to ensure we don't falsely reject requests that could have fit.
	for _, r := range reqs {
		err := c.checkAndReserveRedis(context.Background(), keyHash, r.estCost)
		r.respCh <- err
	}
}

// Reconcile uses asynchronous Micro-Batching to group refunds.
// It flushes to Redis every 100ms, entirely removing reconciliation from the critical path.
func (c *Client) Reconcile(ctx context.Context, keyHash string, reservedCost, actualCost float64) error {
	c.reconcileMu.Lock()
	if c.reconcileBatches == nil {
		c.reconcileBatches = make(map[string]reconcileReq)
		time.AfterFunc(100*time.Millisecond, func() {
			c.flushReconcileBatches()
		})
	}

	existing := c.reconcileBatches[keyHash]
	existing.reservedCost += reservedCost
	existing.actualCost += actualCost
	c.reconcileBatches[keyHash] = existing
	c.reconcileMu.Unlock()

	return nil // Always return immediately (fire-and-forget)
}

func (c *Client) flushReconcileBatches() {
	c.reconcileMu.Lock()
	batches := c.reconcileBatches
	c.reconcileBatches = nil
	c.reconcileMu.Unlock()

	for keyHash, req := range batches {
		go func(kh string, r reconcileReq) {
			err := c.reconcileRedis(context.Background(), kh, r.reservedCost, r.actualCost)
			if err != nil {
				logging.Logger.Error().Err(err).Msg("failed to flush batched reconciliation")
			}
		}(keyHash, req)
	}
}
