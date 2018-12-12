// Description: Some utilities for byterpc
// Author: liming.one@bytedance.com
package byterpc

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isExported(t *testing.T) {
	t.Run("exported", func(t *testing.T) {
		assert.True(t, isExported("FooMethod"))
		assert.True(t, isExported("BarMethod"))
	})

	t.Run("un-exported", func(t *testing.T) {
		assert.False(t, isExported("fooMethod"))
		assert.False(t, isExported("barMethod"))
	})

}

func Test_isExportedOrBuiltinType(t *testing.T) {

	type Foo struct {
		Name string
		Age  int64
	}

	type bar struct {
		Name string
		Age  int64
	}

	t.Run("exported and build-in type", func(t *testing.T) {
		assert.True(t, isExportedOrBuiltinType(reflect.TypeOf(Foo{})))
	})

	t.Run("unexported and build-in type", func(t *testing.T) {
		assert.False(t, isExportedOrBuiltinType(reflect.TypeOf(bar{})))
	})

}

func Test_getLocalIPV4(t *testing.T) {
	t.Run("get local ip ok for mac", func(t *testing.T) {
		ip, err := getLocalIPV4()
		if assert.NoError(t, err) {
			assert.NotEmpty(t, ip)
			fmt.Printf("IP:%s", ip)
		}
	})
}

func TestWrapArrayErrs(t *testing.T) {
	t.Run("wrap multi errors", func(t *testing.T) {
		var errs []error
		errs = append(errs, errors.New("error 1"))
		errs = append(errs, errors.New("error 2"))
		errs = append(errs, errors.New("error 3"))
		expectRs := "error 1\nerror 2\nerror 3\n"
		err := WrapArrayErrs(errs)
		assert.NotEmpty(t, err)
		assert.Equal(t, err.Error(), expectRs)
	})
	t.Run("wrap empty errors", func(t *testing.T) {
		var errs []error
		err := WrapArrayErrs(errs)
		assert.Nil(t, err)
	})

}

func TestMD5Of(t *testing.T) {
	t.Run("right generate", func(t *testing.T) {
		s := "Hello,World!"
		r := MD5Of(s)
		assert.Equal(t, r, "98f97a791ef1457579a5b7e88a495063")
	})
	t.Run("equal test", func(t *testing.T) {
		s := "Hello,World!"
		r := MD5Of(s)
		s2 := "Hello,World!!"
		r2 := MD5Of(s2)
		assert.NotEqual(t, r, r2)
	})
}

func TestTransformInvokePointToPath(t *testing.T) {
	t.Run("right transform", func(t *testing.T) {
		p := "a.b.c:FooService.BarMethod"
		r, _ := TransformInvokePointToPath(p)
		assert.Equal(t, r, "/a.b.c/FooService")
	})

	t.Run("illegal transform", func(t *testing.T) {
		p := "a.b.c.FooService.BarMethod"
		_, e := TransformInvokePointToPath(p)
		assert.Error(t, e, ErrInvalidInvokePoint.Error())
	})
}
