// Description: Service watcher watching service changing
// Author: liming.one@bytedance.com
package byterpc

import (
	"context"
	"encoding/json"

	"github.com/docker/libkv/store"
)

type ServiceWatcher struct {
	key       string
	registry  store.Store
	updates   []*Update
	ctx       context.Context
	cancel    context.CancelFunc
	addressCh <-chan []*store.KVPair
	stopCh    chan struct{}
}

func (w *ServiceWatcher) Close() {
	w.cancel()
	close(w.stopCh)
}

func newServiceWatcher(key string, registry store.Store) Watcher {
	ctx, cancel := context.WithCancel(context.Background())
	w := &ServiceWatcher{
		key:      ServiceRootPath + key,
		registry: registry,
		ctx:      ctx,
		stopCh:   make(chan struct{}),
		cancel:   cancel,
	}

	return w
}

func (w *ServiceWatcher) Next() ([]*Update, error) {
	log.Infof("watching tree for %s", w.key)
	if w.addressCh == nil {
		var watchErr error
		w.addressCh, watchErr = w.registry.WatchTree(w.key, w.stopCh)
		if watchErr != nil {
			return []*Update{}, watchErr
		}
	}
	for {
		select {
		case <-w.stopCh:
			log.Info("watcher has been closed")
			return []*Update{}, nil
		case pairs := <-w.addressCh:
			log.Info("got changes!")
			newUpdates := w.extractUpdates(pairs)
			w.updates = newUpdates
			return w.updates, nil
		}
	}
	return []*Update{}, nil
}

func (w *ServiceWatcher) extractUpdates(resp []*store.KVPair) []*Update {
	var updates []*Update
	for _, n := range resp {
		endpoint := Endpoint{}
		err := json.Unmarshal([]byte(n.Value), &endpoint)
		if err != nil {
			continue
		}
		updates = append(updates, &Update{
			Addr:     endpoint.Address,
			Metadata: &endpoint.MetaData,
		})
	}
	return updates
}
