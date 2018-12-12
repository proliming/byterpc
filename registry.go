package byterpc

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"time"

	"github.com/docker/libkv"
	"github.com/docker/libkv/store"
	"github.com/docker/libkv/store/consul"
	"github.com/docker/libkv/store/etcd"
	"github.com/docker/libkv/store/zookeeper"
)

var (
	ErrUnsupportedRegistrationType = errors.New("unsupported registration type")
)

// Service registry
// etcd, consul,zookeeper ..
type ServiceRegistry interface {
	Init(registryType string, config *RegistryConfig) error

	Register(s *Endpoint) error

	DeRegister(s *Endpoint) error

	DeRegisterAll() error

	ListServices(invokePoint string) []*Endpoint

	Destroy() error
}

type RegistryConfig struct {
	RegistryType      string
	Addrs             []string
	TLS               *tls.Config
	ConnectionTimeout time.Duration
	Bucket            string
	PersistConnection bool
	Username          string
	Password          string
}

type kvRegistry struct {
	storeEngine   store.Store
	backend       store.Backend
	servicesCache map[string]*Endpoint
	rootPath      string
}

func (kvr *kvRegistry) Init(registryType string, config *RegistryConfig) error {
	switch registryType {
	case "etcd":
		kvr.backend = store.ETCD
		kvr.rootPath = ServiceRootPath
		etcd.Register()
	case "consul":
		kvr.backend = store.CONSUL
		kvr.rootPath = ConsulServiceRootPath
		consul.Register()
	case "zookeeper":
		kvr.backend = store.ZK
		kvr.rootPath = ZKServiceRootPath
		zookeeper.Register()
	default:
		log.Errorf("unsupported registryType:%s", registryType)
		return ErrUnsupportedRegistrationType
	}
	if err := checkRegistryConfig(config); err != nil {
		return err
	} else {
		kvr.storeEngine, err = libkv.NewStore(kvr.backend, config.Addrs, WrapKVConfig(config))
		if err != nil {
			log.Errorf("init kvRegistry error: %s by config:%v", err, config)
			return err
		}
	}
	kvr.servicesCache = make(map[string]*Endpoint)
	return nil
}

func checkRegistryConfig(config *RegistryConfig) error {
	return nil
}

func WrapKVConfig(config *RegistryConfig) *store.Config {
	conf := &store.Config{}
	conf.ConnectionTimeout = config.ConnectionTimeout
	conf.Username = config.Username
	conf.Password = config.Password
	conf.PersistConnection = config.PersistConnection
	conf.TLS = config.TLS
	conf.Bucket = config.Bucket
	conf.ClientTLS = &store.ClientTLSConfig{}
	return conf
}

func (kvr *kvRegistry) Register(service *Endpoint) error {
	fullPath := kvr.rootPath + service.Path
	exists, err := kvr.storeEngine.Exists(fullPath)
	if err != nil {
		return err
	}
	if !exists {
		log.Infof("registering service %s ", fullPath)

	} else {
		log.Warnf("registering service %s but already exists, now updating", fullPath)
	}
	kvr.servicesCache[fullPath] = service
	return kvr.storeEngine.Put(fullPath, service.Data(), &store.WriteOptions{})
}

func (kvr *kvRegistry) DeRegister(service *Endpoint) error {
	fullPath := kvr.rootPath + service.Path
	if _, cached := kvr.servicesCache[fullPath]; cached {
		return kvr.storeEngine.Delete(fullPath)
	} else {
		log.Warnf("service %s not exists.", fullPath)
	}
	return nil
}

func (kvr *kvRegistry) DeRegisterAll() error {
	var errs []error
	for path := range kvr.servicesCache {
		err := kvr.storeEngine.Delete(path)
		if err != nil {
			errs = append(errs, err)
		}
	}
	err := WrapArrayErrs(errs)
	if err != nil {
		return err
	}
	return nil
}

func (kvr *kvRegistry) ListServices(invokePoint string) []*Endpoint {
	var services []*Endpoint
	epPath, err := TransformInvokePointToPath(invokePoint)
	if err != nil {
		return services
	}
	pairs, err := kvr.storeEngine.List(kvr.rootPath + epPath)
	if err != nil {
		log.Errorf("list services error:%s for invokePoint:%s", err.Error(), invokePoint)
		return services
	}
	for _, p := range pairs {
		var rs Endpoint
		err = json.Unmarshal(p.Value, &rs)
		if err != nil {
			log.Errorf("unmarshal Endpoint error %s", err.Error())
			continue
		}
		services = append(services, &rs)
	}
	return services
}

func (kvr *kvRegistry) Destroy() error {
	return nil
}
