// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"github.com/docker/libkv/store/zookeeper"
)

// ETCDResolver is an implementation of balance.Resolver
type ZKResolver struct {
	Config *RegistryConfig
}

func NewZKResolver(cfg *RegistryConfig) Resolver {
	return &ZKResolver{Config: cfg}
}

// Resolve to resolve the service from etcd
func (zr *ZKResolver) Resolve(target string) (Watcher, error) {
	if target == "" {
		return nil, ErrNoTargetProvided
	}
	client, err := zookeeper.New(zr.Config.Addrs, WrapKVConfig(zr.Config))
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
