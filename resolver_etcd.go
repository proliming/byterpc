// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"errors"

	"github.com/docker/libkv/store/etcd"
)

var ErrNoTargetProvided = errors.New("no target provided")

// ETCDResolver is an implementation of balance.Resolver
type ETCDResolver struct {
	Config *RegistryConfig
}

func NewETCDResolver(cfg *RegistryConfig) Resolver {
	return &ETCDResolver{Config: cfg}
}

// Resolve to resolve the service from etcd
func (er *ETCDResolver) Resolve(target string) (Watcher, error) {
	if target == "" {
		return nil, ErrNoTargetProvided
	}
	client, err := etcd.New(er.Config.Addrs, WrapKVConfig(er.Config))
	if err != nil {
		return nil, err
	}
	log.Infof("resolving target %s", target)
	key, err := TransformInvokePointToPath(target)
	if err != nil {
		return nil, err
	}
	return newServiceWatcher(key, client), nil
}
