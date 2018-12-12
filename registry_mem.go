// Description:
// Author: liming.one@bytedance.com
package byterpc

import (
	"strings"
	"sync"
)

type memoryRegistry struct {
	serviceLock sync.RWMutex
	cache       map[string]*Endpoint
}

func NewMemoryRegistry() ServiceRegistry {
	return &memoryRegistry{
		cache: make(map[string]*Endpoint),
	}
}

func (r *memoryRegistry) Init(registryType string, config *RegistryConfig) error {
	r.serviceLock.Lock()
	r.cache = make(map[string]*Endpoint)
	r.serviceLock.Unlock()
	return nil
}

func (r *memoryRegistry) Destroy() error {
	r.serviceLock.Lock()
	r.cache = nil
	r.serviceLock.Unlock()
	return nil
}

func (r *memoryRegistry) Register(s *Endpoint) error {
	r.serviceLock.Lock()
	r.cache[s.Path] = s
	r.serviceLock.Unlock()
	return nil
}
func (r *memoryRegistry) DeRegister(s *Endpoint) error {
	r.serviceLock.Lock()
	delete(r.cache, s.Path)
	r.serviceLock.Unlock()
	return nil
}

func (r *memoryRegistry) DeRegisterAll() error {
	r.serviceLock.Lock()
	r.cache = make(map[string]*Endpoint)
	r.serviceLock.Unlock()
	return nil
}

func (r *memoryRegistry) ListServices(invokePoint string) []*Endpoint {
	var services []*Endpoint
	for _, v := range r.cache {
		if strings.Contains(v.Path, invokePoint) {
			services = append(services, v)
		}
	}
	return services
}

// Registry Plugin for Memory
type MemServiceRegistrationPlugin struct {
	registry ServiceRegistry
	disabled bool
}

func (r *MemServiceRegistrationPlugin) Name() string {
	return "_plugin_service_registration_mem"
}
func (r *MemServiceRegistrationPlugin) Type() PluginType {
	return ServiceRegistrationPluginType
}
func (r *MemServiceRegistrationPlugin) IsDisabled() bool {
	return r.disabled
}
func (r *MemServiceRegistrationPlugin) SetDisable(disable bool) {
	r.disabled = disable
}

func (r *MemServiceRegistrationPlugin) RegisterService(service *Endpoint) error {
	return r.registry.Register(service)
}

func (r *MemServiceRegistrationPlugin) DeRegisterService(service *Endpoint) error {
	return r.registry.DeRegister(service)
}
