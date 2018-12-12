package byterpc

// 服务端编解码器，用于实现对rpc请求的解码和rpc响应的编码
type ServerCodec interface {
	ReadRequestHeader(req *RpcRequest) error
	ReadRequestBody(interface{}) error
	WriteResponse(*RpcResponse, interface{}) error
	Close() error
}

// 客户端编解码器,用于实现对rpc响应的编解码以及rpc请求的编码
type ClientCodec interface {
	ReadResponseHeader(*RpcResponse) error
	ReadResponseBody(interface{}) error
	WriteRequest(*RpcRequest, interface{}) error
	Close() error
}
