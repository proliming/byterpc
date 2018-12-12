package byterpc

type RpcRequest struct {
	ServiceMethod string // format: "Service.Method"
	Seq           uint64 // sequence number chosen by client
	next          *RpcRequest
}
