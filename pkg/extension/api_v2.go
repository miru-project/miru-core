package extension

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
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

var Api2Cache = make(map[string]*ExtApiV2)

func (api *ExtApiV2) loadApiV2() {
	api.latest()
}

//	func example() {
//		loop := eventloop.NewEventLoop(
//			eventloop.WithRegistry(SharedRegistry), // 指定模塊註冊表
//		)
//		loop.Run(func(vm *goja.Runtime) {
//			vm.Set(`println`, func(args ...any) {
//				fmt.Println(args...)
//			})
//			var job = Job{loop: loop}
//			vm.Set(`sleep`, func(call goja.FunctionCall) goja.Value {
//				duration := time.Millisecond * time.Duration(call.Argument(0).ToInteger())
//				promise, resolve, _ := vm.NewPromise()
//				// 增加一個等待事件
//				job.Add()
//				// 異步方法
//				go func(duration time.Duration) {
//					time.Sleep(duration)
//					// 需要使用 RunOnLoop 將函數投遞到事件 goroutine 中 resolve/reject
//					loop.RunOnLoop(func(r *goja.Runtime) {
//						// 減少一個等待事件
//						job.Done()
//						// 完成異步方法
//						resolve(time.Now())
//					})
//				}(duration)
//				// 返回 promise
//				return vm.ToValue(promise)
//			})
//			// 腳本入口
//			handlerror(vm.RunString(`
//
// println(1)
//
//	async function test(){
//		await sleep(3000);
//		println("success")
//		}
//
//	sleep(1000).then((now)=>{
//	    println(now)
//	})
//
// println(2)
// test()
// `))
//
//		})
//	}
func (api *ExtApiV2) latest() {

	ser := api.service
	loop := eventloop.NewEventLoop(
		eventloop.WithRegistry(SharedRegistry), // 指定模塊註冊表
	)
	res := make(chan *goja.Promise)
	defer close(res)

	var runtime *goja.Runtime
	loop.RunOnLoop(func(vm *goja.Runtime) {

		//testing  only, get the first key
		var firstKey string
		for k := range Api2Cache {
			firstKey = k
			break
		}
		runtime = vm
		api := Api2Cache[firstKey]
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

		ser.createSingleChannel(vm, "jsRequest", &job, loop, func(call goja.FunctionCall, resolve func(any) error) any {

			url := call.Argument(0).ToString()
			opt := call.Argument(1).ToObject(vm).Export()
			headers := opt.(map[string]any)["headers"]
			log.Println(headers)
			result := make(map[string]string)
			for key, value := range headers.(map[string]any) {
				strValue := value.(string) // Attempt type assertion to string
				result[key] = strValue
			}
			response := handlerror(request(url.String(), result))

			return response
		})
		o := handlerror(vm.RunString("latest(1)"))

		// because it eval async funcion the value become a promise and send to channel
		res <- o.Export().(*goja.Promise)
	})
	loop.Start()
	defer loop.Stop()
	o := handlerror(await[Latests](<-res))
	runtime.Interrupt("exit")
	log.Println(o)

}

// Create a go routine that check Promise is fulfilled or rejected
// and return the result
func await[T any](promise *goja.Promise) (T, error) {
	done := make(chan int)
	var latest T
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
		json.Unmarshal(d, &latest)
		return latest, nil

	default: // case goja.PromiseStateRejected:

		state := promise.State()
		log.Println(state)
		err := promise.Result().Export().(map[string]any)
		e := fmt.Errorf("%q", err)
		return latest, e
	}
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
