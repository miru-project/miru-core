package jsExtension

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/dop251/goja"
	"github.com/miru-project/miru-core/pkg/network"
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

func (ser *ExtBaseService) initFetch(vm *goja.Runtime, job *Job) {
	vm.Set("Request", createRequestCtor(vm))
	vm.Set("Response", createResponseCtor(vm))
	vm.Set("AbortSignal", createAbortSignalCtor())
	vm.Set("AbortController", createAbortControllerCtor(vm))

	// fetch(resource, options)
	ser.createSingleChannel(vm, "fetch", job, func(call goja.FunctionCall, resolve func(any) error) any {
		var fetchUrl string
		requestOptions := network.RequestOptions{
			Headers: make(map[string]string),
			Method:  "GET",
		}

		arg0 := call.Argument(0)
		var optsVal *goja.Object

		if arg0.ExportType().Kind() == reflect.String {
			fetchUrl = arg0.String()
			if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) {
				optsVal = call.Argument(1).ToObject(vm)
			}
		} else if obj, ok := arg0.Export().(map[string]any); ok {
			if u, ok := obj["url"].(string); ok {
				fetchUrl = u
			}
			optsVal = arg0.ToObject(vm)
		} else {
			panic("Miru_core(fetch): resource is not String or Object")
		}

		if optsVal != nil {
			if v := optsVal.Get("method"); v != nil && !goja.IsUndefined(v) {
				requestOptions.Method = v.String()
			}
			if v := optsVal.Get("headers"); v != nil && !goja.IsUndefined(v) {
				if m, ok := v.Export().(map[string]interface{}); ok {
					for k, val := range m {
						requestOptions.Headers[k] = fmt.Sprint(val)
					}
				}
			}
			if v := optsVal.Get("body"); v != nil && !goja.IsUndefined(v) {
				requestOptions.RequestBody = v.String()
			}
			if v := optsVal.Get("timeout"); v != nil && !goja.IsUndefined(v) {
				requestOptions.Timeout = int(v.ToInteger())
			}
		}

		res, err := network.ExtensionRequest(fetchUrl, &requestOptions)
		if err != nil {
			panic(vm.ToValue(err))
		}

		// Create a Response object similar to browser's Response
		responseObj := vm.NewObject()
		responseObj.Set("status", res.StatusCode)
		responseObj.Set("statusText", http.StatusText(res.StatusCode))
		responseObj.Set("ok", res.StatusCode >= 200 && res.StatusCode < 300)
		responseObj.Set("data", res.Body)

		// Add JSON method
		responseObj.Set("json", func() *goja.Promise {
			p, resolve, reject := vm.NewPromise()
			go func() {
				var jsonData interface{}
				err := json.Unmarshal([]byte(res.Body), &jsonData)
				if err != nil {
					job.loop.RunOnLoop(func(vm *goja.Runtime) {
						reject(vm.ToValue(err.Error()))
					})
					return
				}
				job.loop.RunOnLoop(func(vm *goja.Runtime) {
					resolve(vm.ToValue(jsonData))
				})
			}()
			return p
		})

		// Add text method
		responseObj.Set("text", func() *goja.Promise {
			p, resolve, _ := vm.NewPromise()
			job.loop.RunOnLoop(func(vm *goja.Runtime) {
				resolve(vm.ToValue(res.Body))
			})
			return p
		})

		// Add headers
		headers := vm.NewObject()
		for k, v := range res.Headers {
			headers.Set(k, v)
		}
		responseObj.Set("headers", headers)

		return responseObj
	})
}
