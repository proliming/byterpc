package byterpc

type etcdRegistry struct {
	delegate *kvRegistry
}

func NewEtcdRegistry(config *RegistryConfig) ServiceRegistry {
	r := &etcdRegistry{
		delegate: &kvRegistry{},
	}
	r.Init("etcd", config)
	return r
}

func (r *etcdRegistry) Init(registryType string, config *RegistryConfig) error {
	return r.delegate.Init("etcd", config)
}

func (r *etcdRegistry) Register(s *Endpoint) error {
	return r.delegate.Register(s)
}

func (r *etcdRegistry) DeRegister(s *Endpoint) error {
	return r.delegate.DeRegister(s)
}

func (r *etcdRegistry) DeRegisterAll() error {
	return r.delegate.DeRegisterAll()
}

func (r *etcdRegistry) ListServices(endpoint string) []*Endpoint {
	return r.delegate.ListServices(endpoint)
}

func (r *etcdRegistry) Destroy() error {
	return r.delegate.Destroy()
}

// Registry Plugin for ETCD
type EtcdServiceRegistrationPlugin struct {
	registry ServiceRegistry
	disabled bool
}

func (r *EtcdServiceRegistrationPlugin) Name() string {
	return "_plugin_service_registration_etcd"
}
func (r *EtcdServiceRegistrationPlugin) Type() PluginType {
	return ServiceRegistrationPluginType
}
func (r *EtcdServiceRegistrationPlugin) IsDisabled() bool {
	return r.disabled
}
func (r *EtcdServiceRegistrationPlugin) SetDisable(disable bool) {
	r.disabled = disable
}

func (r *EtcdServiceRegistrationPlugin) RegisterService(service *Endpoint) error {
	return r.registry.Register(service)
}

func (r *EtcdServiceRegistrationPlugin) DeRegisterService(service *Endpoint) error {
	return r.registry.DeRegister(service)
}
