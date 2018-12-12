package byterpc

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
)

type serverRequest struct {
	Method string           `json:"method"`
	Params *json.RawMessage `json:"params"`
	Id     *json.RawMessage `json:"id"`
}

func (r *serverRequest) reset() {
	r.Method = ""
	r.Params = nil
	r.Id = nil
}

type serverResponse struct {
	Id     *json.RawMessage `json:"id"`
	Result interface{}      `json:"result"`
	Error  interface{}      `json:"error"`
}

type jsonServerCodec struct {
	dec     *json.Decoder
	enc     *json.Encoder
	clo     io.Closer
	req     serverRequest
	mutex   sync.Mutex
	seq     uint64
	pending map[uint64]*json.RawMessage
}

var null = json.RawMessage([]byte("null"))

func NewJsonServerCodec(conn io.ReadWriteCloser) ServerCodec {
	return &jsonServerCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		clo:     conn,
		pending: make(map[uint64]*json.RawMessage),
	}
}

func (jsc *jsonServerCodec) ReadRequestHeader(r *RpcRequest) error {
	jsc.req.reset()
	if err := jsc.dec.Decode(&jsc.req); err != nil {
		return err
	}
	r.ServiceMethod = jsc.req.Method

	jsc.mutex.Lock()
	jsc.seq++
	jsc.pending[jsc.seq] = jsc.req.Id
	jsc.req.Id = nil
	r.Seq = jsc.seq
	jsc.mutex.Unlock()

	return nil
}

func (jsc *jsonServerCodec) ReadRequestBody(x interface{}) error {
	if x == nil {
		return nil
	}
	if jsc.req.Params == nil {
		return errors.New("illegal argument for request params")
	}
	// JSON params is array value.
	// RPC params is struct.
	// Unmarshal into array containing struct for now.
	// Should think about making RPC more general.
	var params [1]interface{}
	params[0] = x
	return json.Unmarshal(*jsc.req.Params, &params)
}

func (jsc *jsonServerCodec) WriteResponse(r *RpcResponse, x interface{}) error {
	jsc.mutex.Lock()
	b, ok := jsc.pending[r.Seq]
	if !ok {
		jsc.mutex.Unlock()
		return errors.New("invalid sequence number in response")
	}
	delete(jsc.pending, r.Seq)
	jsc.mutex.Unlock()

	if b == nil {
		// Invalid request so no id. Use JSON null.
		b = &null
	}
	resp := serverResponse{Id: b}
	if r.Error == "" {
		resp.Result = x
	} else {
		resp.Error = r.Error
	}
	return jsc.enc.Encode(resp)
}

func (jsc *jsonServerCodec) Close() error {
	return jsc.clo.Close()
}

// 客户端Json编解码器
type jsonClientCodec struct {
	dec *json.Decoder
	enc *json.Encoder
	c   io.Closer

	req  clientRequest
	resp clientResponse

	mutex   sync.Mutex
	pending map[uint64]string // map request id to method name
}

func NewJsonClientCodec(conn io.ReadWriteCloser) ClientCodec {
	return &jsonClientCodec{
		dec:     json.NewDecoder(conn),
		enc:     json.NewEncoder(conn),
		c:       conn,
		pending: make(map[uint64]string),
	}
}

type clientRequest struct {
	Method string         `json:"method"`
	Params [1]interface{} `json:"params"`
	Id     uint64         `json:"id"`
}

func (c *jsonClientCodec) WriteRequest(r *RpcRequest, param interface{}) error {
	c.mutex.Lock()
	c.pending[r.Seq] = r.ServiceMethod
	c.mutex.Unlock()
	c.req.Method = r.ServiceMethod
	c.req.Params[0] = param
	c.req.Id = r.Seq
	return c.enc.Encode(&c.req)
}

type clientResponse struct {
	Id     uint64           `json:"id"`
	Result *json.RawMessage `json:"result"`
	Error  interface{}      `json:"error"`
}

func (r *clientResponse) reset() {
	r.Id = 0
	r.Result = nil
	r.Error = nil
}

func (c *jsonClientCodec) ReadResponseHeader(r *RpcResponse) error {
	c.resp.reset()
	if err := c.dec.Decode(&c.resp); err != nil {
		return err
	}

	c.mutex.Lock()
	r.ServiceMethod = c.pending[c.resp.Id]
	delete(c.pending, c.resp.Id)
	c.mutex.Unlock()

	r.Error = ""
	r.Seq = c.resp.Id
	if c.resp.Error != nil || c.resp.Result == nil {
		x, ok := c.resp.Error.(string)
		if !ok {
			return fmt.Errorf("invalid error %v", c.resp.Error)
		}
		if x == "" {
			x = "unspecified error"
		}
		r.Error = x
	}
	return nil
}

func (c *jsonClientCodec) ReadResponseBody(x interface{}) error {
	if x == nil {
		return nil
	}
	return json.Unmarshal(*c.resp.Result, x)
}

func (c *jsonClientCodec) Close() error {
	return c.c.Close()
}
