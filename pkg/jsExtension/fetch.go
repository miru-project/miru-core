package jsExtension

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/dop251/goja"
	"github.com/miru-project/miru-core/pkg/event"
	"github.com/miru-project/miru-core/pkg/logger"
	"github.com/miru-project/miru-core/pkg/network"
	"github.com/miru-project/miru-core/proto/generate/proto"
)

func createRequestCtor(vm *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		self := call.This
		url := ""
		method := "GET"
		headers := map[string]string{}
		body := ""

		if len(call.Arguments) > 0 {
			arg0 := call.Argument(0)
			if arg0.ExportType().Kind() == reflect.String {
				url = arg0.String()
			} else if obj, ok := arg0.Export().(map[string]interface{}); ok {
				if u, ok := obj["url"].(string); ok {
					url = u
				}
			}
		}
		if len(call.Arguments) > 1 {
			opts := call.Argument(1).ToObject(vm)
			if v := opts.Get("method"); v != nil && !goja.IsUndefined(v) {
				method = v.String()
			}
			if v := opts.Get("headers"); v != nil && !goja.IsUndefined(v) {
				if m, ok := v.Export().(map[string]interface{}); ok {
					for k, val := range m {
						headers[k] = fmt.Sprint(val)
					}
				}
			}
			if v := opts.Get("body"); v != nil && !goja.IsUndefined(v) {
				body = v.String()
			}
		}

		self.Set("url", url)
		self.Set("method", method)
		self.Set("headers", headers)
		self.Set("body", body)
		return self
	}
}

// Helper to create a JS constructor for Response
func createResponseCtor(vm *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		self := call.This
		body := ""
		status := 200
		statusText := "OK"
		headers := map[string]string{}

		if len(call.Arguments) > 0 {
			body = call.Argument(0).String()
		}
		if len(call.Arguments) > 1 {
			opts := call.Argument(1).ToObject(vm)
			if v := opts.Get("status"); v != nil && !goja.IsUndefined(v) {
				status = int(v.ToInteger())
			}
			if v := opts.Get("statusText"); v != nil && !goja.IsUndefined(v) {
				statusText = v.String()
			}
			if v := opts.Get("headers"); v != nil && !goja.IsUndefined(v) {
				if m, ok := v.Export().(map[string]interface{}); ok {
					for k, val := range m {
						headers[k] = fmt.Sprint(val)
					}
				}
			}
		}

		self.Set("body", body)
		self.Set("status", status)
		self.Set("statusText", statusText)
		self.Set("headers", headers)
		self.Set("json", func(call goja.FunctionCall) goja.Value {
			var v interface{}
			err := json.Unmarshal([]byte(body), &v)
			if err != nil {
				panic(vm.ToValue(err.Error()))
			}
			return vm.ToValue(v)
		})
		self.Set("text", func(call goja.FunctionCall) goja.Value {
			return vm.ToValue(body)
		})
		return self
	}
}

func createAbortSignalCtor() func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		self := call.This
		self.Set("aborted", false)
		self.Set("onabort", nil)
		self.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
			// No-op for now
			return goja.Undefined()
		})
		return self
	}
}

func createAbortControllerCtor(vm *goja.Runtime) func(call goja.ConstructorCall) *goja.Object {
	return func(call goja.ConstructorCall) *goja.Object {
		self := call.This
		// Create signal object
		signalObj := vm.NewObject()
		signalObj.Set("aborted", false)
		signalObj.Set("onabort", nil)
		signalObj.Set("addEventListener", func(call goja.FunctionCall) goja.Value {
			// No-op for now
			return goja.Undefined()
		})
		self.Set("signal", signalObj)
		self.Set("abort", func(call goja.FunctionCall) goja.Value {
			signalObj.Set("aborted", true)
			// Call onabort if set
			onabort := signalObj.Get("onabort")
			if onabort != nil && !goja.IsUndefined(onabort) && onabort != goja.Null() {
				if fn, ok := goja.AssertFunction(onabort); ok {
					_, _ = fn(goja.Undefined(), nil)
				}
			}
			return goja.Undefined()
		})
		return self
	}
}

func (api *ExtApi) initFetch(vm *goja.Runtime, job *Job) {
	pkg := api.Ext.Pkg
	vm.Set("Request", createRequestCtor(vm))
	vm.Set("Response", createResponseCtor(vm))
	vm.Set("AbortSignal", createAbortSignalCtor())
	vm.Set("AbortController", createAbortControllerCtor(vm))

	// __fetch(resource, options) - internal Go function
	api.service.createSingleChannel(vm, "__fetch", job, func(call goja.FunctionCall, resolve func(any) error) any {
		var fetchUrl string
		requestOptions := network.RequestOptions{
			Headers: make(map[string]string),
			Method:  "GET",
		}

		arg0 := call.Argument(0)
		var optsVal any

		if arg0.ExportType().Kind() == reflect.String {
			fetchUrl = arg0.String()
			if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) {
				optsVal = call.Argument(1).Export()
			}
		} else if obj, ok := arg0.Export().(map[string]any); ok {
			if u, ok := obj["url"].(string); ok {
				fetchUrl = u
			}
			optsVal = arg0.Export()
		} else {
			panic("Miru_core(fetch): resource is not String or Object")
		}

		if optsVal != nil {
			if m, ok := optsVal.(map[string]any); ok {
				if v, ok := m["method"].(string); ok {
					requestOptions.Method = v
				}
				if v, ok := m["headers"].(map[string]any); ok {
					for k, val := range v {
						requestOptions.Headers[k] = fmt.Sprint(val)
					}
				}
				if v, ok := m["body"].(string); ok {
					requestOptions.RequestBody = v
				}
				if v, ok := m["timeout"].(float64); ok {
					requestOptions.Timeout = int(v)
				} else if v, ok := m["timeout"].(int64); ok {
					requestOptions.Timeout = int(v)
				}
			}
		}

		start := time.Now()
		res, err := network.Request[string](fetchUrl, &requestOptions, network.ReadAll)
		duration := time.Since(start).Milliseconds()

		// Capture event for dev mode if anyone is listening
		if event.GlobalBus.HasSubscribers() {
			status := res.StatusCode
			resHeaders := fmt.Sprintf("%v", res.Headers)

			resBody := res.Body
			if len(resBody) > 100<<10 {
				resBody = resBody[:100<<10]
			}

			go func(s int, h string, b string) {
				event.SendDevNetwork(&proto.DevNetworkEvent{
					Package:         pkg,
					Url:             fetchUrl,
					Method:          requestOptions.Method,
					Status:          int32(s),
					Duration:        duration,
					Timestamp:       time.Now().UnixMilli(),
					RequestHeaders:  fmt.Sprintf("%v", requestOptions.Headers),
					RequestBody:     requestOptions.RequestBody,
					ResponseHeaders: h,
					ResponseBody:    b,
				})
			}(status, resHeaders, resBody)
		}

		if err != nil {
			panic(err.Error())
		}

		headers := res.Headers
		status := res.StatusCode

		return map[string]any{
			"status":     status,
			"statusText": http.StatusText(status),
			"ok":         status >= 200 && status < 300,
			"headers":    headers,
			"data":       res.Body,
			"_isFetch":   true,
		}
	})

	// Define global fetch in JS
	_, err := vm.RunString(`
		globalThis.fetch = async (url, options) => {
			const res = await __fetch(url, options);
			if (res && res._isFetch) {
				return new Response(res.data, {
					status: res.status,
					statusText: res.statusText,
					headers: res.headers
				});
			}
			return res;
		};
	`)
	if err != nil {
		logger.Println("Error setting global fetch:", err)
	}
}
