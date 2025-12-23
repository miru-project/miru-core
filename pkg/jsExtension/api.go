package jsExtension

import (
	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/eventloop"
)

func AsyncCallBack(api *ExtApi, pkg string, evalStr string, loadfunc func(pkg string)) (any, error) {
	api.lock.Lock()
	defer api.lock.Unlock()

	ApiPkgCache.Store(pkg, api)

	var loop *eventloop.EventLoop

	// check extension does  contain eventloop runtime
	lop, eventLoopIsExist := extMemMap.Load(pkg)
	if eventLoopIsExist {
		loop = lop.(*eventloop.EventLoop)
	} else {
		loadfunc(pkg)
		lop, _ = extMemMap.Load(pkg)
		loop = lop.(*eventloop.EventLoop)
	}
	loop.Stop()
	res := make(chan PromiseResult)
	defer close(res)
	loop.RunOnLoop(func(vm *goja.Runtime) {
		o, e := vm.RunString(evalStr)
		handlePromise(o, res, e)
	})
	loop.Start()
	defer loop.StopNoWait()
	result := <-res
	// close(res)
	// handle error from PromiseResult{err: e}
	if result.err != nil {
		var zero any
		return zero, result.err
	}
	// handle result when Promise has established
	o, e := await(result.promise)
	return o, e

}
