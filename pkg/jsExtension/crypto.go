package jsExtension

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"strconv"

	"github.com/dop251/goja"
)

func (ser *ExtBaseService) initCrypto(vm *goja.Runtime) {
	cryptoObj := vm.NewObject()
	cryptoObj.Set("getRandomValues", func(call goja.FunctionCall) goja.Value {
		arg0 := call.Argument(0)
		if goja.IsUndefined(arg0) || goja.IsNull(arg0) {
			panic(vm.ToValue("TypeError: crypto.getRandomValues: argument is missing"))
		}

		obj := arg0.ToObject(vm)
		className := obj.ClassName()
		if className == "Object" {
			// Fallback: Check constructor name
			if ctor := obj.Get("constructor"); ctor != nil {
				if ctorObj := ctor.ToObject(vm); ctorObj != nil {
					if nameVal := ctorObj.Get("name"); nameVal != nil {
						className = nameVal.String()
					}
				}
			}
		}

		lengthVal := obj.Get("length")
		if lengthVal == nil {
			panic(vm.ToValue("TypeError: crypto.getRandomValues: argument is not a TypedArray"))
		}
		length := int(lengthVal.ToInteger())

		var bytesPerElement int
		switch className {
		case "Int8Array", "Uint8Array", "Uint8ClampedArray":
			bytesPerElement = 1
		case "Int16Array", "Uint16Array":
			bytesPerElement = 2
		case "Int32Array", "Uint32Array":
			bytesPerElement = 4
		case "BigInt64Array", "BigUint64Array":
			bytesPerElement = 8
		default:
			panic(vm.ToValue(fmt.Sprintf("TypeError: crypto.getRandomValues: unsupported type %s", className)))
		}

		totalBytes := length * bytesPerElement
		// Chrome limit is 65536 bytes.
		if totalBytes > 65536 {
			panic(vm.ToValue("QuotaExceededError: crypto.getRandomValues: request too large"))
		}

		b := make([]byte, totalBytes)
		_, err := rand.Read(b)
		if err != nil {
			panic(vm.ToValue("Error generating random values"))
		}

		for i := 0; i < length; i++ {
			var val interface{}
			start := i * bytesPerElement
			end := start + bytesPerElement
			chunk := b[start:end]

			switch className {
			case "Int8Array":
				val = int8(chunk[0])
			case "Uint8Array", "Uint8ClampedArray":
				val = uint8(chunk[0])
			case "Int16Array":
				val = int16(binary.LittleEndian.Uint16(chunk))
			case "Uint16Array":
				val = uint16(binary.LittleEndian.Uint16(chunk))
			case "Int32Array":
				val = int32(binary.LittleEndian.Uint32(chunk))
			case "Uint32Array":
				val = uint32(binary.LittleEndian.Uint32(chunk))
			case "BigInt64Array":
				val = int64(binary.LittleEndian.Uint64(chunk))
			case "BigUint64Array":
				val = uint64(binary.LittleEndian.Uint64(chunk))
			}

			obj.Set(strconv.Itoa(i), val)
		}

		return arg0
	})

	vm.Set("crypto", cryptoObj)
}
