package byterpc

type RpcResponse struct {
	ServiceMethod string // echoes that of the Request
	Seq           uint64 // echoes that of the request
	Error         string // error, if any.
	next          *RpcResponse
}
