package result

type Result[T any] struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    T      `json:"data"`
	Code    int    `json:"code"`
}

func NewResult(success bool, message string, data any, code int) *Result[any] {
	return &Result[any]{
		Success: success,
		Message: message,
		Data:    data,
		Code:    code,
	}
}

func NewSuccessResult(data any) *Result[any] {
	return NewResult(true, "success", data, 200)
}

func NewErrorResult(message string, code int) *Result[any] {
	return NewResult(false, message, "", code)
}
