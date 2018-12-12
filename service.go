package byterpc

import (
	"encoding/json"
	"errors"
	"reflect"
	"sync"
)

const (
	ServiceRootPath       = "/byterpc/services"
	ConsulServiceRootPath = "byterpc/services"
	ZKServiceRootPath     = "byterpc/services"
)

var (
	ErrMethodMustHaveThreeParameters  = errors.New("method to be exported must has 2 parameters and 1 error return value")
	ErrMethodArgsMustBeExported       = errors.New("method's args must be exported")
	ErrMethodSecondArgMustBePointer   = errors.New("method's second args must be pointer")
	ErrMethodReturnValueMustBeAnError = errors.New("method's return value must be an error")
)

// A group of method
type Service struct {
	name        string
	aliasName   string
	receiver    reflect.Value
	receiverTyp reflect.Type
	sync.Mutex
	methods map[string]*Method
}

type Method struct {
	sync.Mutex
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint
}

// Endpoint is the wrap of Service for registration
type Endpoint struct {
	PSM         string            `json:"psm"` // Product.Service.Module  unique
	Address     string            `json:"address"`
	ServiceName string            `json:"service_name"`
	Path        string            `json:"path"` //  /PSM/ServiceName/IP:PORT
	MetaData    map[string]string // eg. the vm info or env
}

func (rs Endpoint) Data() []byte {
	bts, _ := json.Marshal(rs.MetaData)
	return bts
}

// Register methods
func registerMethods(typ reflect.Type, skipError bool) (map[string]*Method, error) {
	methods := make(map[string]*Method)
	for m := 0; m < typ.NumMethod(); m++ {
		method := typ.Method(m)
		methodType := method.Type
		methodName := method.Name
		// Method must be exported.
		if method.PkgPath != "" {
			continue
		}
		// Method needs three ins: receiver, *args, *reply.
		if methodType.NumIn() != 3 {
			if skipError {
				log.Infof("method %s has %d input parameters; needs exactly three\n", methodName, methodType.NumIn())
			} else {
				return nil, ErrMethodMustHaveThreeParameters
			}
			continue
		}
		// First arg need not be a pointer.
		argType := methodType.In(1)

		if !isExportedOrBuiltinType(argType) {
			if skipError {
				log.Infof("request type of method %s is not exported: %q\n", methodName, argType)
			} else {
				return nil, ErrMethodArgsMustBeExported
			}
			continue
		}
		// Second arg must be a pointer.
		replyType := methodType.In(2)
		if replyType.Kind() != reflect.Ptr {
			if skipError {
				log.Infof("reply type of method %s is not a pointer: %q\n", methodName, replyType)
			} else {
				return nil, ErrMethodSecondArgMustBePointer
			}
			continue
		}
		// Reply type must be exported.
		if !isExportedOrBuiltinType(replyType) {
			if skipError {
				log.Infof("reply type of method %s is not exported: %q\n", methodName, replyType)
			} else {
				return nil, ErrMethodArgsMustBeExported
			}
			continue
		}
		// Method needs one out.
		if methodType.NumOut() != 1 {
			if skipError {
				log.Infof("method %s has %d output parameters; needs exactly one\n", methodName, methodType.NumOut())
			} else {
				return nil, ErrMethodReturnValueMustBeAnError
			}
			continue
		}
		// The return type of the method must be error.
		if returnType := methodType.Out(0); returnType != typeOfError {
			if skipError {
				log.Infof("return type of method %s is %q, must be error\n", methodName, returnType)
			} else {
				return nil, ErrMethodReturnValueMustBeAnError
			}
			continue
		}
		methods[methodName] = &Method{method: method, ArgType: argType, ReplyType: replyType}
	}
	return methods, nil
}

// Call the specific method and return the result
func (s *Service) call(server *Server, sending *sync.Mutex, wg *sync.WaitGroup, method *Method, req *RpcRequest, argv, replyVal reflect.Value, codec ServerCodec) {
	if wg != nil {
		defer wg.Done()
	}
	method.Lock()
	method.numCalls++
	method.Unlock()
	function := method.method.Func
	// Invoke the method, providing a new value for the reply.
	returnValues := function.Call([]reflect.Value{s.receiver, argv, replyVal})
	// The return value for the method is an error.
	errInter := returnValues[0].Interface()
	errMsg := ""
	if errInter != nil {
		errMsg = errInter.(error).Error()
	}
	server.sendResponse(sending, req, replyVal.Interface(), codec, errMsg)
	server.freeRequest(req)
}
