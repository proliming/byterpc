package byterpc

import (
	"io"
	"net"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
)

var typeOfError = reflect.TypeOf((*error)(nil)).Elem()

var log = logrus.New()

var (
	ErrInvalidServiceName            = errors.New("invalid service name, can't be empty")
	ErrMethodMustBeExported          = errors.New("method must be exported for registration")
	ErrServiceAlreadyDefinedInServer = errors.New("service already defined in this server.")
	ErrServerCanNotDecodeRequest     = errors.New("server cannot decode request")
)

// A value sent as a placeholder for the server's response value when the server
// receives an invalid request. It is never decoded by the client since the Response
// contains an error when it is used.
var invalidRequest = struct{}{}

type ServerListener func(server *Server)

// Server is the server side of byterpc
type Server struct {
	serverConfig  *ServerConfig
	ln            net.Listener
	pluginManager PluginManager

	acLock     sync.RWMutex
	activeConn map[net.Conn]struct{}

	serviceMap sync.Map // map[string]*Service
	reqLock    sync.Mutex
	freeReq    *RpcRequest
	respLock   sync.Mutex
	freeResp   *RpcResponse

	serveWG       sync.WaitGroup // counts active Serve goroutines for GracefulStop
	stopListeners []ServerListener
	stopChan      chan bool
	errChan       chan error
}

var serverInstance *Server
var serverOnce sync.Once

// New server with the specific config.
func NewServer(conf *ServerConfig) *Server {
	serverOnce.Do(func() {
		serverInstance = newRpcServerWithOptions(conf)
	})
	return serverInstance
}

// GetServer always return a singleton Server Instance.
func GetServer() *Server {
	return serverInstance
}

// Start the server
func (server *Server) Start() {
	if server.ln == nil {
		panic("server's net listener can't be nil")
	}
	server.acLock.Lock()
	if server.activeConn == nil {
		server.activeConn = map[net.Conn]struct{}{}
	}
	server.acLock.Unlock()

	server.acceptLoop()
}

func (server *Server) acceptLoop() {
	for {
		select {
		case <-server.stopChan:
			log.Warnf("Stopping server...")
			server.onStop()
		default:
		}

		conn, err := server.ln.Accept()
		if err != nil {
			log.Errorf("accepted client connection error :%s", err.Error())
			// TODO block if error happened?
			// server.errChan <- err
			continue
		}
		conn, er := server.pluginManager.HandleConnAccept(conn)
		if er != nil {
			log.Errorf("handle conn accept by plugin error:%s", err)
			// TODO block if error happened?
			// server.errChan <- err
			continue
		}
		log.Infof("connection from: %s established.", conn.RemoteAddr())

		// Add a new active conn
		server.acLock.Lock()
		server.activeConn[conn] = struct{}{}
		server.acLock.Unlock()

		server.serveWG.Add(1)
		// Start a new goroutine to deal with Conn
		go func() {
			defer server.serveWG.Done()
			server.handleConn(conn)
		}()
	}
}

// RegisterService publishes in the server the set of methods of the
// receiver value that satisfy the following conditions:
//	- exported method of exported type
//	- two arguments, both of exported type
//	- the second argument is a pointer
//	- one return value, of type error
// It returns an error if the receiver is not an exported type or has
// no suitable methods. It also logs the error using package log.
// The client accesses each method using a string of the form "Type.Method",
// where Type is the receiver's concrete type.
func (server *Server) RegisterService(receiver interface{}) error {
	return server.register(receiver, "", false)
}

// RegisterServiceWithName is like Register but uses the provided name for the type
// instead of the receiver's concrete type.
func (server *Server) RegisterServiceWithName(receiver interface{}, name string) error {
	if len(name) == 0 {
		return ErrInvalidServiceName
	}
	return server.register(receiver, name, true)
}

func (server *Server) register(receiver interface{}, alias string, useAlias bool) error {

	s := &Service{}
	s.receiverTyp = reflect.TypeOf(receiver)
	s.receiver = reflect.ValueOf(receiver)
	sName := reflect.Indirect(s.receiver).Type().Name()
	if useAlias {
		sName = alias
	}
	if sName == "" {
		return ErrInvalidServiceName
	}
	if !isExported(sName) && !useAlias {
		return ErrMethodMustBeExported
	}
	s.name = strings.Trim(sName, " ")

	log.Infof("registering service: %s ", s.name)

	methods, err := registerMethods(s.receiverTyp, false)
	if err != nil {
		return err
	}
	s.methods = methods

	if _, dup := server.serviceMap.LoadOrStore(sName, s); dup {
		return ErrServiceAlreadyDefinedInServer
	}

	// Do plugin register
	if rs, err := server.endpointOf(s); err == nil {
		server.pluginManager.RegisterService(rs)
	}
	return nil
}

// handleConn forks a goroutine to handle a just-accepted connection that
// has not had any I/O performed on it yet.
// handleConn blocks, serving the connection until the client hangs up.
func (server *Server) handleConn(conn net.Conn) {
	var serverCodec ServerCodec
	switch server.serverConfig.CodecType {
	case "json":
		serverCodec = NewJsonServerCodec(conn)
	case "gob":
		serverCodec = NewGobServerCodec(conn)
	default:
		// log error
	}
	if serverCodec != nil {
		server.serveCodec(serverCodec)
	}
	if conn, ok := conn.(net.Conn); ok {
		server.acLock.Lock()
		delete(server.activeConn, conn)
		server.acLock.Unlock()
	}
	err := server.pluginManager.HandleConnClose(conn.(net.Conn))
	if err != nil {
		log.Errorf("handle conn close error:%s", err)
		// server.errChan <- err
	}
}

// ServeCodec is like ServeConn but uses the specified codec to
// decode requests and encode responses.
func (server *Server) serveCodec(codec ServerCodec) {
	sending := new(sync.Mutex)
	wg := new(sync.WaitGroup)
	for {
		service, method, req, argv, replyVal, keepReading, err := server.readRequest(codec)
		if err != nil {
			// server.errChan <- err
			if !keepReading {
				break
			}
			if req != nil {
				server.sendResponse(sending, req, invalidRequest, codec, err.Error())
				server.freeRequest(req)
			}
			continue
		}
		wg.Add(1)
		go service.call(server, sending, wg, method, req, argv, replyVal, codec)
	}
	// We've seen that there are no more requests.
	// Wait for responses to be sent before closing
	wg.Wait()
	// Close the codec & conn
	codec.Close()
}

func (server *Server) readRequest(codec ServerCodec) (service *Service, method *Method, req *RpcRequest, argv, replyVal reflect.Value, keepReading bool, err error) {

	service, method, req, keepReading, err = server.readRequestHeader(codec)
	if err != nil {
		if !keepReading {
			return
		}
		// discard body
		codec.ReadRequestBody(nil)
		return
	}

	// Decode the argument value.
	argIsValue := false // if true, need to indirect before calling.
	if method.ArgType.Kind() == reflect.Ptr {
		argv = reflect.New(method.ArgType.Elem())
	} else {
		argv = reflect.New(method.ArgType)
		argIsValue = true
	}
	// argv guaranteed to be a pointer now.
	if err = codec.ReadRequestBody(argv.Interface()); err != nil {
		return
	}
	if argIsValue {
		argv = argv.Elem()
	}

	replyVal = reflect.New(method.ReplyType.Elem())

	switch method.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyVal.Elem().Set(reflect.MakeMap(method.ReplyType.Elem()))
	case reflect.Slice:
		replyVal.Elem().Set(reflect.MakeSlice(method.ReplyType.Elem(), 0, 0))
	}

	if err != nil {
		return
	}
	if err = server.pluginManager.PostReadRequest(req, argv); err != nil {
		return
	}
	return
}

func (server *Server) readRequestHeader(codec ServerCodec) (svc *Service, method *Method, req *RpcRequest, keepReading bool, err error) {
	// Grab the request header.
	req = server.getRequest()
	err = codec.ReadRequestHeader(req)
	if err != nil {
		req = nil
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return
		}
		err = ErrServerCanNotDecodeRequest
		return
	}
	// We read the header successfully. If we see an error now,
	// we can still recover and move on to the next request.
	keepReading = true

	dot := strings.LastIndex(req.ServiceMethod, ".")
	if dot < 0 {
		err = errors.New("service/method request invalid: " + req.ServiceMethod)
		return
	}
	serviceName := req.ServiceMethod[:dot]
	methodName := req.ServiceMethod[dot+1:]

	// Look up the request.
	serv, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("can't find service " + req.ServiceMethod)
		return
	}
	svc = serv.(*Service)
	method = svc.methods[methodName]
	if method == nil {
		err = errors.New("can't find method " + req.ServiceMethod)
	}
	if err != nil {
		return
	}
	if err = server.pluginManager.PreReadRequest(req); err != nil {
		return
	}
	return

}

func (server *Server) sendResponse(sending *sync.Mutex, req *RpcRequest, reply interface{}, codec ServerCodec, errMsg string) {

	resp := server.getResponse()

	defer func() {
		server.pluginManager.PostWriteResponse(resp, reply)
		sending.Unlock()
		server.freeResponse(resp)
	}()

	// Encode the response header
	resp.ServiceMethod = req.ServiceMethod
	if errMsg != "" {
		resp.Error = errMsg
		reply = invalidRequest
	}
	resp.Seq = req.Seq
	sending.Lock()

	log.Infof("sending response %v", reply)

	server.pluginManager.PreWriteResponse(resp, reply)

	wErr := codec.WriteResponse(resp, reply)
	if wErr != nil {
		errMsg = wErr.Error()
	}

}

func (server *Server) getResponse() *RpcResponse {
	server.respLock.Lock()
	resp := server.freeResp
	if resp == nil {
		resp = new(RpcResponse)
	} else {
		server.freeResp = resp.next
		*resp = RpcResponse{}
	}
	server.respLock.Unlock()
	return resp
}

func (server *Server) freeResponse(resp *RpcResponse) {
	server.respLock.Lock()
	resp.next = server.freeResp
	server.freeResp = resp
	server.respLock.Unlock()
}

func (server *Server) getRequest() *RpcRequest {
	server.reqLock.Lock()
	req := server.freeReq
	if req == nil {
		req = new(RpcRequest)
	} else {
		server.freeReq = req.next
		*req = RpcRequest{}
	}
	server.reqLock.Unlock()
	return req
}

func (server *Server) freeRequest(req *RpcRequest) {
	server.reqLock.Lock()
	req.next = server.freeReq
	server.freeReq = req
	server.reqLock.Unlock()
}

func (server *Server) MainPort() string {
	return strconv.Itoa(server.serverConfig.Port)
}
func (server *Server) PSM() string {
	return server.serverConfig.PSM
}

func newRpcServerWithOptions(conf *ServerConfig) *Server {
	s := &Server{
		serverConfig:  conf,
		pluginManager: newServerPluginManager(),
		stopChan:      make(chan bool),
		errChan:       make(chan error, 16),
	}
	s.init()
	return s
}

func (server *Server) init() {
	server.check()
	server.initListener()
}

func (server *Server) check() {
	if err := server.serverConfig.check(); err != nil {
		panic("server config error:" + err.Error())
	}
}

func (server *Server) initListener() {

	log.Infof("start listening on tcp@localhost:%s", server.MainPort())
	ln, err := net.Listen("tcp", ":"+server.MainPort())
	if err != nil {
		panic("server: listening tcp port " + server.MainPort() + " with error:" + err.Error())
	}
	server.ln = ln

}

func (server *Server) endpointOf(s *Service) (*Endpoint, error) {
	r := &Endpoint{}
	r.ServiceName = s.name
	r.PSM = server.serverConfig.PSM
	r.Address = server.ln.Addr().String()
	r.Path = "/" + r.PSM + "/" + r.ServiceName + "/" + r.Address
	return r, nil

}
func (server *Server) onStop() {
	server.serveWG.Wait()
	for _, l := range server.stopListeners {
		l(server)
	}
}
