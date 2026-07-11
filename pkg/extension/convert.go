package extension

import (
	"encoding/base64"
	"errors"
	"reflect"

	"github.com/go-viper/mapstructure/v2"
)

// Unmarshal decodes an arbitrary value (typically produced by a script
// runtime such as goja) into a value of type T, honouring the `json` tag and
// converting byte slices into base64-encoded strings.
//
// It is intentionally runtime-agnostic so that both the JS and Golang
// extension runtimes can share it without creating an import cycle.
func Unmarshal[T any](input any) (*T, error) {
	var result T
	config := &mapstructure.DecoderConfig{
		Metadata: nil,
		Result:   &result,
		TagName:  "json",
		DecodeHook: func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
			if f.Kind() == reflect.Slice && f.Elem().Kind() == reflect.Uint8 && t.Kind() == reflect.String {
				return base64.StdEncoding.EncodeToString(data.([]uint8)), nil
			}
			return data, nil
		},
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, err
	}
	err = decoder.Decode(input)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UnmarshalList decodes a list of arbitrary values into a slice of *T.
func UnmarshalList[T any](input any) ([]*T, error) {
	items, ok := input.([]any)
	if !ok {
		return nil, errors.New("input is not a list")
	}
	result := make([]*T, len(items))
	for i, item := range items {
		u, err := Unmarshal[T](item)
		if err != nil {
			return nil, err
		}
		result[i] = u
	}
	return result, nil
}
