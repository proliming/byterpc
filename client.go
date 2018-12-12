package byterpc

import (
	"errors"
	"io"
	"net"
	"sync"
)

var ErrShutdown = errors.New("connection is shut down")

type ServerError string

func (e ServerError) Error() string {
	return string(e)
}

// Call represents an active RPC.
type Call struct {
	ServiceMethod string // ServiceName.MethodName
	Args          interface{}
	Reply         interface{}
	Error         error
	Done          chan *Call
}

// Client represents an RPC Client.
type Client struct {
	codec ClientCodec

	pluginManager PluginManager
	reqMutex      sync.Mutex // protects following
	request       RpcRequest
	mutex         sync.Mutex // protects following
	seq           uint64
	pending       map[uint64]*Call
	closing       bool // user has called Close
	shutdown      bool // server has told us to stop
}

// NewClient returns a new Client to handle requests to the
// set of services at the other end of the connection.
func NewClient(conn io.ReadWriteCloser) *Client {
	c := NewGobClientCodec(conn)
	return NewClientWithCodec(c)
}

// NewClientWithCodec is like NewClient but uses the specified
// codec to encode requests and decode responses.
func NewClientWithCodec(cc ClientCodec) *Client {

	c := &Client{
		codec:         cc,
		pending:       make(map[uint64]*Call),
		pluginManager: newClientPluginManager(),
	}
	go c.receive()
	return c

}

// Dial connects to an RPC server at the specified network address and return a Client
func Connect(network, address string) (*Client, error) {
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return NewClient(conn), nil
}

func (client *Client) Close() error {
	client.mutex.Lock()
	if client.closing {
		client.mutex.Unlock()
		return ErrShutdown
	}
	client.closing = true
	client.mutex.Unlock()
	return client.codec.Close()
}

// Go invokes the function asynchronously. It returns the Call structure representing
// the invocation. The done channel will signal when the call is complete by returning
// the same Call object. If done is nil, Go will allocate a new channel.
// If non-nil, done must be buffered or Go will deliberately crash.
func (client *Client) Go(serviceMethod string, args interface{}, reply interface{}, done chan *Call) *Call {
	call := new(Call)
	call.ServiceMethod = serviceMethod
	call.Args = args
	call.Reply = reply

	/// If caller passes done != nil, it must arrange that
	// done has enough buffer for the number of simultaneous
	// RPCs that will be using that channel. If the channel
	// is totally unbuffered, it's best not to run at all.
	if done == nil {
		done = make(chan *Call, 10) // buffered.
	} else {
		if cap(done) == 0 {
			log.Panic("done channel is unbuffered")
		}
	}

	call.Done = done
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args interface{}, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}

func (client *Client) send(call *Call) {
	client.reqMutex.Lock()
	defer client.reqMutex.Unlock()

	// Register this call.
	client.mutex.Lock()
	if client.shutdown || client.closing {
		client.mutex.Unlock()
		call.Error = ErrShutdown
		call.done()
		return
	}
	seq := client.seq
	client.seq++
	client.pending[seq] = call
	client.mutex.Unlock()

	// Encode and send the request.
	client.request.Seq = seq
	client.request.ServiceMethod = call.ServiceMethod
	err := client.codec.WriteRequest(&client.request, call.Args)
	if err != nil {
		client.mutex.Lock()
		call = client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()
		if call != nil {
			call.Error = err
			call.done()
		}
	}
}

// Loop for receiving server response.
func (client *Client) receive() {
	var err error
	var response RpcResponse
	for err == nil {
		response = RpcResponse{}
		err = client.codec.ReadResponseHeader(&response)
		if err != nil {
			log.Errorf("read response header error %s", err)
			break
		}
		seq := response.Seq
		client.mutex.Lock()
		call := client.pending[seq]
		delete(client.pending, seq)
		client.mutex.Unlock()

		// call plugins
		err = client.pluginManager.PreReadResponse(&response)
		if err != nil {
			break
		}

		switch {
		case call == nil:
			// We've got no pending call. That usually means that
			// WriteRequest partially failed, and call was already
			// removed; response is a server telling us about an
			// error reading request body. We should still attempt
			// to read error body, but there's no one to give it to.
			err = client.codec.ReadResponseBody(nil)
			if err != nil {
				err = errors.New("reading error body: " + err.Error())
			}
		case response.Error != "":
			// We've got an error response. Give this to the request;
			// any subsequent requests will get the ReadResponseBody
			// error if there is one.
			call.Error = ServerError(response.Error)
			err = client.codec.ReadResponseBody(nil)
			if err != nil {
				err = errors.New("reading error body: " + err.Error())
			}
			call.done()
		default:
			err = client.codec.ReadResponseBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body " + err.Error())
			}
			err = client.pluginManager.PostReadResponse(&response, call.Reply)
			if err != nil {
				err = errors.New("execute client PostReadResponse error:" + err.Error())
			}
			call.done()
		}
	}
	// Terminate pending calls.
	client.reqMutex.Lock()
	client.mutex.Lock()
	client.shutdown = true
	closing := client.closing
	if err == io.EOF {
		if closing {
			err = ErrShutdown
		} else {
			err = io.ErrUnexpectedEOF
		}
	}
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
	client.mutex.Unlock()
	client.reqMutex.Unlock()
	if err != io.EOF && !closing {
		log.Infof("client protocol error:%s", err)
	}
}

func (call *Call) done() {
	select {
	case call.Done <- call:
		// ok
	default:
		// We don't want to block here. It is the caller's responsibility to make
		// sure the channel has enough buffer space. See comment in Go().
		log.Infof("discarding Call reply due to insufficient Done chan capacity")
	}
}
