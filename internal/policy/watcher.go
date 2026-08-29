package policy

import (
	"context"
	"time"

	"github.com/try-loopers/loopers-oss/internal/logging"
	"github.com/fsnotify/fsnotify"
)

func (e *Engine) StartWatcher(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	err = watcher.Add(e.cfg.PolicyDir)
	if err != nil {
		watcher.Close()
		return err
	}

	go func() {
		defer watcher.Close()
		var timer *time.Timer

		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) || event.Has(fsnotify.Rename) {
					// Debounce reloads (wait 500ms for consecutive changes to settle)
					if timer != nil {
						timer.Stop()
					}
					timer = time.AfterFunc(500*time.Millisecond, func() {
						if err := e.Reload(); err != nil {
							logging.Logger.Error().Err(err).Msg("Failed to reload policies on file change")
						}
					})
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				logging.Logger.Error().Err(err).Msg("Policy watcher error")
			}
		}
	}()

	return nil
}
