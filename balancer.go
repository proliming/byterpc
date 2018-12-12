// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrClientConnClosing = errors.New("balancer is closed")
var DefaultSelector = NewRandomSelector()

// Address represents a server the client connects to.
type Address struct {
	// Addr is the server address on which a connection will be established.
	// IP:PORT
	Addr string
	// Metadata is the information associated with Addr, which may be used
	// to make load balancing decision.
	Metadata interface{}
}

// Balancer chooses network addresses for RPCs.
type Balancer interface {
	// Start does the initialization work to bootstrap a Balancer. For example,
	// this function may start the name resolution and watch the updates. It will
	// be called when dialing.
	Start(target string) error

	// Up informs the Balancer that RPC has a connection to the server at
	// addr. It returns down which is called once the connection to addr gets
	// lost or closed.
	// Up(addr Address) (down func(error))
	// Get gets the address of a server for the RPC corresponding to ctx.
	// i) If it returns a connected address, RPC internals issues the RPC on the
	// connection to this address;
	// ii) If it returns an address on which the connection is under construction
	// (initiated by Notify(...)) but not connected, RPC internals
	//  * fails RPC if the RPC is fail-fast and connection is in the TransientFailure or
	//  Shutdown state;
	//  or
	//  * issues RPC on the connection otherwise.
	// iii) If it returns an address on which the connection does not exist, RPC
	// internals treats it as an error and will fail the corresponding RPC.
	//
	// Therefore, the following is the recommended rule when writing a custom Balancer.
	// If blockingWait is true, it should return a connected address or
	// block if there is no connected address. It should respect the timeout or
	// cancellation of ctx when blocking. If blockingWait is false (for fail-fast
	// RPCs), it should return an address it has notified via Notify(...) immediately
	// instead of blocking.
	//
	// The function returns put which is called once the rpc has completed or failed.
	// put can collect and report RPC stats to a remote load balancer.
	//
	// This function should only return the errors Balancer cannot recover by itself.
	// RPC internals will fail the RPC if an error is returned.
	Get(ctx context.Context, blockingWait bool) (addr Address, put func(), err error)
	// Notify returns a channel that is used by RPC internals to watch the addresses
	// RPC needs to connect. The addresses might be from a name resolver or remote
	// load balancer. byterpc internals will compare it with the existing connected
	// addresses. If the address Balancer notified is not in the existing connected
	// addresses, byterpc starts to connect the address. If an address in the existing
	// connected addresses is not in the notification list, the corresponding connection
	// is shutdown gracefully. Otherwise, there are no operations to take. Note that
	// the Address slice must be the full list of the Addresses which should be connected.
	// It is NOT delta.
	Notify() <-chan []Address
	// Close shuts down the balancer.
	Close() error
}

type balancer struct {
	r      Resolver
	w      Watcher
	s      Selector
	mu     sync.Mutex
	addrCh chan []Address // the channel to notify rpc internals the list of addresses the client should connect to.
	waitCh chan struct{}  // the channel to block when there is no connected address available
	done   bool           // The Balancer is closed.
}

func NewBalancer(r Resolver, s Selector) Balancer {
	if s == nil {
		s = DefaultSelector
	}
	return &balancer{r: r, s: s}
}

func (b *balancer) watchAddrUpdates() error {
	updates, err := b.w.Next()
	if err != nil {
		log.Infof("the naming watcher stops working due to %v.\n", err)
		return err
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	var updatedAddrs []Address
	for _, u := range updates {
		updatedAddrs = append(updatedAddrs, Address{
			Addr:     u.Addr,
			Metadata: u.Metadata,
		})
	}
	log.Infof("updating addresses len:%d", len(updatedAddrs))
	b.s.Reset(updatedAddrs)

	if b.done {
		return ErrClientConnClosing
	}
	select {
	case <-b.addrCh:
	default:
	}
	b.addrCh <- b.s.AddrList()
	return nil
}

func (b *balancer) Start(target string) error {
	log.Infof("starting balancer for target:%s", target)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.done {
		return ErrClientConnClosing
	}
	if b.r == nil {
		return nil
	}
	w, err := b.r.Resolve(target)
	if err != nil {
		return err
	}
	b.w = w
	b.addrCh = make(chan []Address, 1)
	go func() {
		for {
			if err := b.watchAddrUpdates(); err != nil {
				return
			}
		}
	}()
	return nil
}

// Up sets the connected state of addr and sends notification if there are pending
// Get() calls.
/*func (b *balancer) Up(addr Address) func(error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cnt, connected := b.s.Up(addr)
	if connected {
		return func(err error) {
			b.down(addr, err)
		}
	}
	// addr is only one which is connected. Notify the Get() callers who are blocking.
	if cnt == 1 && b.waitCh != nil {
		close(b.waitCh)
		b.waitCh = nil
	}
	return func(err error) {
		b.down(addr, err)
	}
}

// down unsets the connected state of addr.
func (b *balancer) down(addr Address, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.s.Down(addr)
}*/

// Get returns the next addr in the rotation.
func (b *balancer) Get(ctx context.Context, blockingWait bool) (addr Address, put func(), err error) {
	log.Info("getting new address from balancer.")
	b.mu.Lock()
	if b.done {
		b.mu.Unlock()
		err = ErrClientConnClosing
		return
	}

	if blockingWait {
		for {
			if len(b.s.AddrList()) == 0 {
				time.Sleep(time.Millisecond)
			} else {
				break
			}
		}
	}

	addr, err = b.s.Select(ctx)
	if err == nil {
		b.mu.Unlock()
		put = func() {
			b.s.Put(addr.Addr)
		}
		log.Infof("selected %s", addr.Addr)
		return
	}
	return

	/*// Wait on b.waitCh for non-failfast RPCs.
	if b.waitCh == nil {
		ch = make(chan struct{})
		b.waitCh = ch
	} else {
		ch = b.waitCh
	}
	b.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			err = ctx.Err()
			return
		case <-ch:
			b.mu.Lock()
			if b.done {
				b.mu.Unlock()
				err = ErrClientConnClosing
				return
			}

			addr, err = b.s.Select(ctx)
			if err == nil {
				put = func() {
					b.s.Put(addr.Addr)
				}
				b.mu.Unlock()
				return
			}

			// The newly added addr got removed by Down() again.
			if b.waitCh == nil {
				ch = make(chan struct{})
				b.waitCh = ch
			} else {
				ch = b.waitCh
			}
			b.mu.Unlock()
		}
	}*/
}

func (b *balancer) Notify() <-chan []Address {
	return b.addrCh
}

func (b *balancer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.done = true
	if b.w != nil {
		b.w.Close()
	}
	if b.waitCh != nil {
		close(b.waitCh)
		b.waitCh = nil
	}
	if b.addrCh != nil {
		close(b.addrCh)
	}
	return nil
}
