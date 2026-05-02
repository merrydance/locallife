package ordinaryserviceprovider

import (
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

var _ OrdinaryServiceProviderClientInterface = (*Client)(nil)

func TestOrdinaryServiceProviderClientInterfaceDoesNotExposeSDKTypes(t *testing.T) {
	interfaceType := reflect.TypeOf((*OrdinaryServiceProviderClientInterface)(nil)).Elem()

	for methodIndex := 0; methodIndex < interfaceType.NumMethod(); methodIndex++ {
		method := interfaceType.Method(methodIndex)
		for inputIndex := 0; inputIndex < method.Type.NumIn(); inputIndex++ {
			require.NotContains(t, typePackagePath(method.Type.In(inputIndex)), "github.com/wechatpay-apiv3/wechatpay-go", method.Name)
		}
		for outputIndex := 0; outputIndex < method.Type.NumOut(); outputIndex++ {
			require.NotContains(t, typePackagePath(method.Type.Out(outputIndex)), "github.com/wechatpay-apiv3/wechatpay-go", method.Name)
		}
	}
}

func TestOrdinaryServiceProviderClientInterfaceUsesOnlyModuleContracts(t *testing.T) {
	interfaceType := reflect.TypeOf((*OrdinaryServiceProviderClientInterface)(nil)).Elem()

	for methodIndex := 0; methodIndex < interfaceType.NumMethod(); methodIndex++ {
		method := interfaceType.Method(methodIndex)
		for inputIndex := 0; inputIndex < method.Type.NumIn(); inputIndex++ {
			require.NotContains(t, typePackagePath(method.Type.In(inputIndex)), "github.com/merrydance/locallife/wechat/contracts", method.Name)
		}
		for outputIndex := 0; outputIndex < method.Type.NumOut(); outputIndex++ {
			require.NotContains(t, typePackagePath(method.Type.Out(outputIndex)), "github.com/merrydance/locallife/wechat/contracts", method.Name)
		}
	}
}

func typePackagePath(value reflect.Type) string {
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Slice || value.Kind() == reflect.Array {
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		return typePackagePath(value.Key()) + ";" + typePackagePath(value.Elem())
	}
	return strings.TrimSpace(value.PkgPath())
}
