package result

type Result[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
	Code    int    `json:"code"`
}

func NewResult[T any](success bool, message string, data T, code int) *Result[T] {
	return &Result[T]{
		Success: success,
		Message: message,
		Data:    data,
		Code:    code,
	}
}

func NewSuccessResult[T any](data T) *Result[T] {
	return NewResult(true, "success", data, 200)
}

func NewErrorResult[T any](message string, code int, errData T) *Result[T] {
	return NewResult(false, message, errData, code)
}

func NewErrorResultAny(message string, code int) *Result[any] {
	return NewResult[any](false, message, nil, code)
}
