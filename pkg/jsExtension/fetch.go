package jsExtension

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/dop251/goja"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
	"github.com/miru-project/miru-core/pkg/network"
)

type RequestOptions struct {
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Url     string            `json:"url"`
	Timeout int               `json:"timeout"`
}

type Response struct {
	StatusCode int
	StatusText string
	Body       string
	Headers    map[string]string
}

func Request[T any](url string, options *RequestOptions, responseHandler func(*http.Response) (T, error)) (Response, error) {
	if options == nil {
		options = &RequestOptions{
			Method: "GET",
		}
	}

	if options.Method == "" {
		options.Method = "GET"
	}
	log.Println("Request URL:", url)
	client := &http.Client{}
	if options.Timeout > 0 {
		client.Timeout = time.Duration(options.Timeout) * time.Millisecond
	}

	req, err := http.NewRequest(options.Method, url, nil)
	if err != nil {
		return Response{}, err
	}

	if options.Body != "" && (options.Method == "POST" || options.Method == "PUT" || options.Method == "PATCH") {
		req.Body = io.NopCloser(strings.NewReader(options.Body))
	}

	// Set headers
	if options.Headers != nil {
		for key, value := range options.Headers {
			req.Header.Set(key, value)
		}
	}

	// Set default headers if not present
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	}

	resp, err := client.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()

	bodyContent, err := responseHandler(resp)
	if err != nil {
		return Response{}, err
	}

	headers := make(map[string]string)
	for k, v := range resp.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	var bodyStr string
	switch v := any(bodyContent).(type) {
	case string:
		bodyStr = v
	default:
		bodyStr = fmt.Sprintf("%v", v)
	}
	return Response{
		StatusCode: resp.StatusCode,
		StatusText: resp.Status,
		Body:       bodyStr,
		Headers:    headers,
	}, nil
}

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
		var requestOptions RequestOptions

		arg0 := call.Argument(0)

		if arg0.ExportType().Kind() == reflect.String {
			fetchUrl = arg0.String()
		} else if obj, ok := arg0.Export().(map[string]any); ok {

			jsonData := errorhandle.HandleFatal(json.Marshal(obj))

			if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
				panic("Error unmarshalling JSON:" + err.Error())
			}
			fetchUrl = requestOptions.Url
		} else {
			panic("Miru_core(fetch): resource is not String or Object")
		}
		// if _, e := httpurl.Parse(fetchUrl); e != nil {
		// 	panic(fmt.Sprintf("url is not valid: %s", fetchUrl))
		// }

		// if len(call.Arguments) > 1 && !goja.IsUndefined(call.Argument(1)) {
		// 	opt := call.Argument(1).ToObject(vm).Export()
		// 	jsonData := handlerror(json.Marshal(opt))
		// 	if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
		// 		panic("Error unmarshalling JSON:" + err.Error())
		// 	}
		// } else {
		// 	requestOptions.Method = "GET"
		// }

		res, err := Request(fetchUrl, &requestOptions, network.ReadAll)
		if err != nil {
			panic(vm.ToValue(err))
		}

		// Create a Response object similar to browser's Response
		responseObj := vm.NewObject()
		responseObj.Set("status", res.StatusCode)
		responseObj.Set("statusText", res.StatusText)
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
