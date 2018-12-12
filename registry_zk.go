package byterpc

type zkRegistry struct {
	delegate *kvRegistry
}

func NewZKRegistry(config *RegistryConfig) ServiceRegistry {
	r := &zkRegistry{
		delegate: &kvRegistry{},
	}
	r.Init("zookeeper", config)
	return r
}

func (r *zkRegistry) Init(registryType string, config *RegistryConfig) error {
	return r.delegate.Init("zookeeper", config)
}

func (r *zkRegistry) Register(s *Endpoint) error {
	return r.delegate.Register(s)
}

func (r *zkRegistry) DeRegister(s *Endpoint) error {
	return r.delegate.DeRegister(s)
}

func (r *zkRegistry) DeRegisterAll() error {
	return r.delegate.DeRegisterAll()
}

func (r *zkRegistry) ListServices(endpoint string) []*Endpoint {
	return r.delegate.ListServices(endpoint)
}

func (r *zkRegistry) Destroy() error {
	return r.delegate.Destroy()
}

// Registry Plugin for Zookeeper
type ZKServiceRegistrationPlugin struct {
	registry ServiceRegistry
	disabled bool
}

func (r *ZKServiceRegistrationPlugin) Name() string {
	return "_plugin_service_registration_zk"
}
func (r *ZKServiceRegistrationPlugin) Type() PluginType {
	return ServiceRegistrationPluginType
}
func (r *ZKServiceRegistrationPlugin) IsDisabled() bool {
	return r.disabled
}
func (r *ZKServiceRegistrationPlugin) SetDisable(disable bool) {
	r.disabled = disable
}

func (r *ZKServiceRegistrationPlugin) RegisterService(service *Endpoint) error {
	return r.registry.Register(service)
}

func (r *ZKServiceRegistrationPlugin) DeRegisterService(service *Endpoint) error {
	return r.registry.DeRegister(service)
}
