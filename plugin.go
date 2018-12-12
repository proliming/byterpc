package byterpc

import (
	"errors"
	"net"
)

// 不支持的操作类型错误
var UnsupportedOperationError = errors.New("unsuppported operation")

// 插件是byterpc提供了多种扩展点
// Server端： 1. Client建立连接后，断开连接前 2.RPC请求读取前、读取后 3.RPC请求响应前、响应后 4.服务注册前，注册后
// Client端： 1. 与Server建立连接前后 2. RPC请求发送前后 3. RPC响应读取前后
type Plugin interface {
	Name() string
	Type() PluginType
	IsDisabled() bool
	SetDisable(disable bool)
}

type PluginType uint8

const (
	ServerRWPluginType PluginType = iota + 1
	ServerConnPluginType
	ServiceRegistrationPluginType
	ClientRWPluginType
	ClientMonitorPluginType
	ServerMonitorPluginType
)

type (
	ClientRWPlugin interface {
		Plugin
		PreWriteRequest(req *RpcRequest, args interface{}) error
		PostWriteRequest(req *RpcRequest, args interface{}) error
		PreReadResponse(resp *RpcResponse) error
		PostReadResponse(resp *RpcResponse, body interface{}) error
	}

	ServerRWPlugin interface {
		Plugin
		PreReadRequest(req *RpcRequest) error
		PostReadRequest(req *RpcRequest, body interface{}) error
		PreWriteResponse(resp *RpcResponse, body interface{}) error
		PostWriteResponse(resp *RpcResponse, body interface{}) error
	}

	ServerConnPlugin interface {
		Plugin
		HandleConnAccept(net.Conn) (net.Conn, error)
		HandleConnClose(net.Conn) error
	}

	ServiceRegistrationPlugin interface {
		Plugin
		RegisterService(service *Endpoint) error
		DeRegisterService(service *Endpoint) error
	}

	ClientMonitorPlugin interface {
		Plugin
		PreWriteRequest(req *RpcRequest, args interface{}) error
		PostWriteRequest(req *RpcRequest, args interface{}) error
		PreReadResponse(resp *RpcResponse) error
		PostReadResponse(resp *RpcResponse, body interface{}) error
	}

	ServerMonitorPlugin interface {
		Plugin
		PreReadRequest(req *RpcRequest) error
		PostReadRequest(req *RpcRequest, body interface{}) error

		PreWriteResponse(resp *RpcResponse, body interface{}) error
		PostWriteResponse(resp *RpcResponse, body interface{}) error

		HandleConnAccept(net.Conn) (net.Conn, error)
		HandleConnClose(net.Conn) error

		RegisterService(service *Endpoint) error
	}
)

type PluginManager interface {
	Add(p Plugin)

	Remove(p Plugin)

	Disable(p Plugin) error

	Enable(p Plugin) error

	PreWriteRequest(req *RpcRequest, args interface{}) error
	PostWriteRequest(req *RpcRequest, args interface{}) error

	PreReadResponse(resp *RpcResponse) error
	PostReadResponse(resp *RpcResponse, body interface{}) error

	PreReadRequest(req *RpcRequest) error
	PostReadRequest(req *RpcRequest, body interface{}) error

	PreWriteResponse(resp *RpcResponse, body interface{}) error
	PostWriteResponse(resp *RpcResponse, body interface{}) error

	HandleConnAccept(net.Conn) (net.Conn, error)
	HandleConnClose(net.Conn) error

	RegisterService(service *Endpoint) error
}

// ======================================= serverPluginManager =================================== //

func newServerPluginManager() PluginManager {
	return &serverPluginManager{}
}

func newClientPluginManager() PluginManager {
	return &clientPluginManager{}
}

type pluginManager struct {
	plugins map[PluginType]map[string]Plugin
}

func (pm *pluginManager) Add(p Plugin) {
	if typMap, ok := pm.plugins[p.Type()]; ok {
		if typMap == nil {
			typMap = make(map[string]Plugin)
		}
		typMap[p.Name()] = p
	}
}
func (pm *pluginManager) Remove(pl Plugin) {
	if typMap, ok := pm.plugins[pl.Type()]; ok {
		if p, exists := typMap[pl.Name()]; exists {
			delete(typMap, p.Name())
		}
	}
}

func (pm *pluginManager) Disable(pl Plugin) error {
	if typMap, ok := pm.plugins[pl.Type()]; ok {
		if p, exists := typMap[pl.Name()]; exists {
			p.SetDisable(true)
		} else {
			return errors.New("not found plugin, please add it first")
		}
	}
	return nil
}

func (pm *pluginManager) Enable(pl Plugin) error {

	if typMap, ok := pm.plugins[pl.Type()]; ok {
		if p, exists := typMap[pl.Name()]; exists {
			p.SetDisable(false)
		} else {
			return errors.New("not found plugin, please add it first")
		}
	}
	return nil
}

type serverPluginManager struct {
	pluginManager
}

func (spm *serverPluginManager) PreWriteRequest(req *RpcRequest, args interface{}) error {
	return UnsupportedOperationError
}
func (spm *serverPluginManager) PostWriteRequest(req *RpcRequest, args interface{}) error {
	return UnsupportedOperationError
}
func (spm *serverPluginManager) PreReadResponse(resp *RpcResponse) error {
	return UnsupportedOperationError
}
func (spm *serverPluginManager) PostReadResponse(resp *RpcResponse, body interface{}) error {
	return UnsupportedOperationError
}

func (spm *serverPluginManager) PreReadRequest(req *RpcRequest) error {
	serverRWPlugins, exists := spm.plugins[ServerRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range serverRWPlugins {
		rwp := p.(ServerRWPlugin)
		log.Debugf("%s do pre-read-request.", n)
		err := rwp.PreReadRequest(req)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("pre-read-request error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}

func (spm *serverPluginManager) PostReadRequest(req *RpcRequest, body interface{}) error {
	serverRWPlugins, exists := spm.plugins[ServerRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range serverRWPlugins {
		rwp := p.(ServerRWPlugin)
		log.Debugf("%s do post-read-request.", n)
		err := rwp.PostReadRequest(req, body)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("post-read-request error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (spm *serverPluginManager) PreWriteResponse(resp *RpcResponse, body interface{}) error {
	serverRWPlugins, exists := spm.plugins[ServerRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range serverRWPlugins {
		rwp := p.(ServerRWPlugin)
		log.Debugf("%s do pre-write-response.", n)
		err := rwp.PreWriteResponse(resp, body)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("pre-write-response error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (spm *serverPluginManager) PostWriteResponse(resp *RpcResponse, body interface{}) error {
	serverRWPlugins, exists := spm.plugins[ServerRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range serverRWPlugins {
		rwp := p.(ServerRWPlugin)
		log.Debugf("%s do post-write-response.", n)
		err := rwp.PostWriteResponse(resp, body)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("post-write-response error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (spm *serverPluginManager) HandleConnAccept(conn net.Conn) (net.Conn, error) {

	serverHCPlugins, exists := spm.plugins[ServerConnPluginType]
	if !exists {
		return conn, nil
	}
	var errs []error
	for n, p := range serverHCPlugins {
		rwp := p.(ServerConnPlugin)
		log.Debugf("%s do handle-conn-accept.", n)
		cnn, err := rwp.HandleConnAccept(conn)
		if err != nil {
			errs = append(errs, err)
		} else {
			conn = cnn
		}
	}
	if len(errs) != 0 {
		return conn, errors.New("handle-conn-accept error happened: " + WrapArrayErrs(errs).Error())
	}
	return conn, nil
}
func (spm *serverPluginManager) HandleConnClose(conn net.Conn) error {
	serverHCPlugins, exists := spm.plugins[ServerConnPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range serverHCPlugins {
		rwp := p.(ServerConnPlugin)
		log.Debugf("%s do handle-conn-close.", n)
		err := rwp.HandleConnClose(conn)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("handle-conn-close error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}

func (spm *serverPluginManager) RegisterService(s *Endpoint) error {
	if v, exists := spm.plugins[ServiceRegistrationPluginType]; exists {
		for _, p := range v {
			err := p.(ServiceRegistrationPlugin).RegisterService(s)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ======================================= clientPluginManager =================================== //

type clientPluginManager struct {
	pluginManager
}

func (cpm *clientPluginManager) PreWriteRequest(req *RpcRequest, args interface{}) error {
	clientRWPlugins, exists := cpm.plugins[ClientRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range clientRWPlugins {
		rwp := p.(ClientRWPlugin)
		log.Debugf("%s do pre-write-request.", n)
		err := rwp.PreWriteRequest(req, args)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("pre-write-request error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (cpm *clientPluginManager) PostWriteRequest(req *RpcRequest, args interface{}) error {
	clientRWPlugins, exists := cpm.plugins[ClientRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range clientRWPlugins {
		rwp := p.(ClientRWPlugin)
		log.Debugf("%s do post-write-request.", n)
		err := rwp.PostWriteRequest(req, args)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("post-write-request error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (cpm *clientPluginManager) PreReadResponse(resp *RpcResponse) error {
	clientRWPlugins, exists := cpm.plugins[ClientRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range clientRWPlugins {
		rwp := p.(ClientRWPlugin)
		log.Debugf("%s do pre-read-response.", n)
		err := rwp.PreReadResponse(resp)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("pre-read-response error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}
func (cpm *clientPluginManager) PostReadResponse(resp *RpcResponse, body interface{}) error {
	clientRWPlugins, exists := cpm.plugins[ClientRWPluginType]
	if !exists {
		return nil
	}
	var errs []error
	for n, p := range clientRWPlugins {
		rwp := p.(ClientRWPlugin)
		log.Debugf("%s do post-read-response.", n)
		err := rwp.PostReadResponse(resp, body)
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.New("post-read-response error happened: " + WrapArrayErrs(errs).Error())
	}
	return nil
}

func (cpm *clientPluginManager) PreReadRequest(req *RpcRequest) error {
	return UnsupportedOperationError
}

func (cpm *clientPluginManager) PostReadRequest(req *RpcRequest, body interface{}) error {
	return UnsupportedOperationError
}
func (cpm *clientPluginManager) PreWriteResponse(resp *RpcResponse, body interface{}) error {
	return UnsupportedOperationError
}
func (cpm *clientPluginManager) PostWriteResponse(resp *RpcResponse, body interface{}) error {
	return UnsupportedOperationError
}
func (cpm *clientPluginManager) HandleConnAccept(conn net.Conn) (net.Conn, error) {
	return conn, UnsupportedOperationError
}
func (cpm *clientPluginManager) HandleConnClose(net.Conn) error {
	return UnsupportedOperationError
}
func (cpm *clientPluginManager) PreRegister(service *Endpoint) (*Endpoint, error) {
	return service, UnsupportedOperationError
}
func (cpm *clientPluginManager) RegisterService(s *Endpoint) error {
	return UnsupportedOperationError
}
