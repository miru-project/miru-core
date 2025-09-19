package jsExtension

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/dop251/goja"
	errorhandle "github.com/miru-project/miru-core/errorHandle"
)

// Create a go routine that check Promise is fulfilled or rejected
// and return the result
func await[T any](promise *goja.Promise) (T, error) {
	done := make(chan int)
	var dataOut T
	go func() {
		defer close(done)

		for promise.State() == goja.PromiseStatePending {
			time.Sleep(50 * time.Millisecond)
		}

	}()
	<-done
	switch promise.State() {
	case goja.PromiseStateFulfilled:

		o := promise.Result().Export()
		d := errorhandle.HandleFatal(json.Marshal(o))
		json.Unmarshal(d, &dataOut)

		return dataOut, nil

	default: // case goja.PromiseStateRejected:

		state := promise.State()
		log.Println(state)
		res := promise.Result().(*goja.Object)

		err := res.GetOwnPropertyNames()
		errStr := "Js exception:"
		for _, v := range err {
			errStr += fmt.Sprintln(v, ":", res.Get(v).String())
		}

		return dataOut, errors.New(errStr)
	}
}

// Promise for creating a single channel
func (ser *ExtBaseService) resolvePromise(resolve func(any) error, reason any, job *Job) {

	job.Done()
	resolve(reason)

}
func (ser *ExtBaseService) rejectPromise(reject func(any) error, reason any, job *Job) {

	job.Done()
	reject(reason)
}

// Create a single channel
func (ser *ExtBaseService) createSingleChannel(vm *goja.Runtime, name string, job *Job, fun func(call goja.FunctionCall, resolve func(any) error) any) {

	vm.Set(name, func(call goja.FunctionCall) goja.Value {
		promise, resolve, reject := vm.NewPromise()
		// 增加一個等待事件
		job.Add()
		// 異步方法
		go func() {

			defer func() {
				if r := recover(); r != nil {
					log.Println("Recovered from panic:", r)
					ser.rejectPromise(reject, r, job)
				}
			}()

			// Use RunOnLoop to dispatch function to event goroutine
			reason := fun(call, resolve)
			ser.resolvePromise(resolve, reason, job)

		}()
		// 返回 promise
		return vm.ToValue(promise)
	})
}
