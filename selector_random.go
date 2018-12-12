// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"context"
	"math/rand"
	"time"
)

type RandomSelector struct {
	baseSelector
	r *rand.Rand
}

func NewRandomSelector() Selector {
	return &RandomSelector{
		r:            rand.New(rand.NewSource(time.Now().UnixNano())),
		baseSelector: baseSelector{addrMap: make(map[string]*AddrInfo)},
	}
}

func (r *RandomSelector) Select(ctx context.Context) (addr Address, err error) {
	log.Info("random selecting address")

	/*if ctx.Value("blockIfEmpty") != nil {
		for {
			if len(r.addrs) == 0 {
				time.Sleep(time.Millisecond)
			}
		}
	}*/

	if len(r.addrs) == 0 {
		return addr, ErrAddrListEmptyErr
	}

	size := len(r.addrs)
	idx := r.r.Int() % size
	for i := 0; i < size; i++ {
		addr := r.addrs[(idx+i)%size]
		if addrInfo, ok := r.addrMap[addr]; ok {
			if addrInfo.connected {
				addrInfo.load++
				return addrInfo.addr, nil
			}
		}
	}
	return addr, ErrNoAvailableAddressErr
}
