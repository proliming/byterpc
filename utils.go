// Description: Some utilities for byterpc
// Author: liming.one@bytedance.com
package byterpc

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"net"
	"reflect"
	"strings"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidInvokePoint = errors.New("invalid invoke-point, must be: PSM:ServiceName.MethodName")
var Md5Hasher = md5.New()

// upper case - name?
func isExported(methodName string) bool {
	r, _ := utf8.DecodeRuneInString(methodName)
	return unicode.IsUpper(r)
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return isExported(t.Name()) || t.PkgPath() == ""
}

func getLocalIPV4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		if ip, ok := a.(*net.IPNet); ok && !ip.IP.IsLoopback() {
			if ip.IP.To4() != nil {
				return ip.IP.String(), nil
			}
		}
	}
	return "", nil
}

func WrapArrayErrs(errs []error) error {
	sb := strings.Builder{}
	for _, e := range errs {
		sb.WriteString(e.Error() + "\n")
	}
	rs := sb.String()
	if len(strings.Trim(rs, " ")) == 0 {
		return nil
	}
	return errors.New(rs)
}

func MD5Of(text string) string {
	defer Md5Hasher.Reset()
	Md5Hasher.Write([]byte(text))
	return hex.EncodeToString(Md5Hasher.Sum(nil))
}

func TransformInvokePointToPath(invokePoint string) (string, error) {
	sp := strings.Split(invokePoint, ":")
	if len(sp) != 2 {
		return "", ErrInvalidInvokePoint
	}
	psm := sp[0]
	sName := strings.Split(sp[1], ".")[0]
	return "/" + psm + "/" + sName, nil

}
