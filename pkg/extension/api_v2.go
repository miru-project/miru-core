package extension

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
	"github.com/miru-project/miru-core/pkg/network"
)

type ExtApiV2 struct {
	ext     *Ext
	service *ExtBaseService
}
type Job struct {
	loop  *eventloop.EventLoop
	flag  *eventloop.Interval
	count uint64
}

type PromiseResult struct {
	promise *goja.Promise
	err     error
}

func (j *Job) Add() {
	j.count++
	if j.count == 1 {
		j.flag = j.loop.SetInterval(func(r *goja.Runtime) {}, time.Hour*24*365*100)
	}
}
func (j *Job) Done() {
	j.count--
	if j.count == 0 {
		j.loop.ClearInterval(j.flag)
		j.flag = nil
	}
}

var ApiPkgCacheV2 = make(map[string]*ExtApiV2)

func LoadApiV2(ext *Ext, script string) {
	scriptV2 := fmt.Sprintf(script, ext.pkg, ext.name, ext.website)
	compile := handlerror(goja.Compile(ext.pkg+".js", *ext.context, true))
	runtimeV2 := handlerror(goja.Compile("runtime_v2.js", scriptV2, true))
	api := &ExtApiV2{ext: ext, service: &ExtBaseService{program: compile, base: runtimeV2}}

	ApiPkgCacheV2[ext.pkg] = api
	// api.latest(ext.pkg, 1)
}

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
		d := handlerror(json.Marshal(o))
		json.Unmarshal(d, &dataOut)
		return dataOut, nil

	default: // case goja.PromiseStateRejected:

		state := promise.State()
		log.Println(state)
		err := promise.Result().Export()
		e := fmt.Errorf("%q", err)
		return dataOut, e
	}
}

// Handle any extension async callback like latest, search, watch etc
func AsyncCallBackV2[T any](api *ExtApiV2, pkg string, evalStr string) (T, error) {
	ApiPkgCacheV2[pkg] = api
	ser := api.service
	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(SharedRegistry), // 指定模塊註冊表
	)
	res := make(chan PromiseResult)
	defer close(res)

	// var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		service := api.service
		reg := SharedRegistry.Enable(vm)
		initModule(reg, vm)

		// Run the program that has compiled before
		vm.RunProgram(service.base)
		vm.RunProgram(api.service.program)

		vm.Set(`println`, func(args ...any) {
			log.Println(args...)
		})
		var job = Job{loop: loop}

		ser.createSingleChannel(vm, "jsRequest", &job, func(call goja.FunctionCall, resolve func(any) error) any {

			url := call.Argument(0).ToString().String()
			opt := call.Argument(1).ToObject(vm).Export()
			var requestOptions network.RequestOptions

			jsonData := handlerror(json.Marshal(opt))
			if err := json.Unmarshal(jsonData, &requestOptions); err != nil {
				panic("Error unmarshalling JSON:" + err.Error())
			}

			res, err := network.Request(url, &requestOptions)

			if err != nil {
				panic(vm.ToValue(err))
			}
			return res
		})
		o, e := vm.RunString(evalStr)

		if e != nil {
			// This kind of error happens before the async function is called
			res <- PromiseResult{err: e}
		} else {
			// Because it eval async funcion the value become a promise and send to channel
			res <- PromiseResult{promise: o.Export().(*goja.Promise)}
		}

	})
	loop.Start()
	defer loop.Stop()

	result := <-res
	// handle error from PromiseResult{err: e}
	if result.err != nil {
		var zero T
		return zero, result.err
	}
	// handle result when Promise has established
	o, e := await[T](result.promise)
	return o, e

}

// func (api *ExtApiV2) asyncExample() {
// 	ser := api.service
// 	loop := eventloop.NewEventLoop(
// 		eventloop.WithRegistry(SharedRegistry), // 指定模塊註冊表
// 	)
// 	loop.Run(func(vm *goja.Runtime) {
// 		vm.Set(`println`, func(args ...any) {
// 			log.Println(args...)
// 		})
// 		var job = Job{loop: loop}
// 		ser.createSingleChannel(vm, "sleep", &job, loop, func(call goja.FunctionCall, resolve func(any) error) any {
// 			duration := time.Millisecond * time.Duration(call.Argument(0).ToInteger())
// 			time.Sleep(duration)
// 			return time.Now()
// 		})
// 		handlerror(vm.RunString(`
// println(1)
// async function test(){
// 	await sleep(3000);
// 	println("success")
// 	}
// sleep(1000).then((now)=>{
//     println(now)
// })
// println(2)
// test()
// `))
// 	})

// }
