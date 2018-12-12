package byterpc

import (
	"bufio"
	"encoding/gob"
	"io"
)

type gobServerCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
	closed bool
}

func NewGobServerCodec(conn io.ReadWriteCloser) ServerCodec {
	buf := bufio.NewWriter(conn)
	return &gobServerCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(conn),
		encBuf: buf,
		closed: false,
	}
}

func (gsc *gobServerCodec) ReadRequestHeader(r *RpcRequest) error {
	return gsc.dec.Decode(r)
}

func (gsc *gobServerCodec) ReadRequestBody(body interface{}) error {
	return gsc.dec.Decode(body)
}

func (gsc *gobServerCodec) WriteResponse(r *RpcResponse, body interface{}) error {
	if err := gsc.enc.Encode(r); err != nil {
		if gsc.encBuf.Flush() == nil {
			// Gob couldn't encode the header. Should not happen, so if it does,
			// shut down the connection to signal that the connection is broken.
			gsc.Close()
		}
		return err
	}
	if err := gsc.enc.Encode(body); err != nil {
		if gsc.encBuf.Flush() == nil {
			// Was a gob problem encoding the body but the header has been written.
			// Shut down the connection to signal that the connection is broken.
			gsc.Close()
		}
		return err
	}
	return gsc.encBuf.Flush()
}

func (gsc *gobServerCodec) Close() error {
	if gsc.closed {
		// Only call gsc.rwc.Close once; otherwise the semantics are undefined.
		return nil
	}
	gsc.closed = true
	return gsc.rwc.Close()
}

// 客户端Gob编解码器
type gobClientCodec struct {
	rwc    io.ReadWriteCloser
	dec    *gob.Decoder
	enc    *gob.Encoder
	encBuf *bufio.Writer
}

func NewGobClientCodec(conn io.ReadWriteCloser) ClientCodec {
	buf := bufio.NewWriter(conn)
	return &gobClientCodec{
		rwc:    conn,
		dec:    gob.NewDecoder(conn),
		enc:    gob.NewEncoder(conn),
		encBuf: buf,
	}

}

func (c *gobClientCodec) WriteRequest(r *RpcRequest, body interface{}) (err error) {
	if err = c.enc.Encode(r); err != nil {
		return
	}
	if err = c.enc.Encode(body); err != nil {
		return
	}
	return c.encBuf.Flush()
}

func (c *gobClientCodec) ReadResponseHeader(r *RpcResponse) error {
	return c.dec.Decode(r)
}

func (c *gobClientCodec) ReadResponseBody(body interface{}) error {
	return c.dec.Decode(body)
}

func (c *gobClientCodec) Close() error {
	return c.rwc.Close()
}
