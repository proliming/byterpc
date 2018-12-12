// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"context"
	"errors"
	"strconv"
)

type AddrInfo struct {
	addr      Address
	weight    int    //load weight
	load      uint64 //current number of requests
	connected bool
}

type Selector interface {
	Reset([]Address) error
	//Up(addr Address) (cnt int, connected bool)
	//Down(addr Address) error
	AddrList() []Address
	Select(ctx context.Context) (Address, error)
	Put(addr string) error
}

var (
	ErrAddrListEmptyErr      = errors.New("addr list is emtpy")
	ErrAddrDoesNotExistErr   = errors.New("addr does not exist")
	ErrNoAvailableAddressErr = errors.New("no available address")
)

type baseSelector struct {
	addrs   []string
	addrMap map[string]*AddrInfo
}

func (b *baseSelector) Reset(addrs []Address) error {
	b.addrMap = make(map[string]*AddrInfo)
	for _, addr := range addrs {
		if a, ok := b.addrMap[addr.Addr]; ok {
			log.Warnf("addr exist for %s", a.addr.Addr)
			continue
		}
		weight := 1
		m, ok := addr.Metadata.(*map[string]string)
		if ok {
			w, ok := (*m)["weight"]
			if ok {
				n, err := strconv.Atoi(w)
				if err == nil && n > 0 {
					weight = n
				}
			}
		}
		load := 0
		m, ok = addr.Metadata.(*map[string]string)
		if ok {
			w, ok := (*m)["load"]
			if ok {
				n, err := strconv.Atoi(w)
				if err == nil && n > 0 {
					load = n
				}
			}
		}

		b.addrMap[addr.Addr] = &AddrInfo{addr: addr, weight: weight, connected: false, load: uint64(load)}

		// weight
		for i := 0; i < weight; i++ {
			b.addrs = append(b.addrs, addr.Addr)
		}
	}
	return nil
}

/*
func (b *baseSelector) Up(addr Address) (cnt int, connected bool) {

	a, ok := b.addrMap[addr.Addr]
	if ok {
		if a.connected {
			return cnt, true
		}
		a.connected = true
	}
	for _, v := range b.addrMap {
		if v.connected {
			cnt++
			if cnt > 1 {
				break
			}
		}
	}
	return cnt, false
}

func (b *baseSelector) Down(addr Address) error {
	a, ok := b.addrMap[addr.Addr]
	if ok {
		a.connected = false
	} else {
		return ErrAddrDoesNotExistErr
	}
	return nil
}
*/
func (b *baseSelector) AddrList() []Address {
	var list []Address
	for _, v := range b.addrMap {
		list = append(list, v.addr)
	}
	return list
}

func (b *baseSelector) Select(ctx context.Context) (addr Address, err error) {
	return
}

func (b *baseSelector) Put(addr string) error {
	a, ok := b.addrMap[addr]
	if ok {
		a.load--
	}
	return nil
}
