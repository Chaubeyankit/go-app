package response

import "github.com/ankit.chaubey/myapp/pkg/apperrors"

type Envelope struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrBody    `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type ErrBody struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"` // for validation errors
}

type Meta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	TotalItems int64 `json:"totalItems"`
	TotalPages int   `json:"totalPages"`
}

func OK(data interface{}) Envelope {
	return Envelope{Success: true, Data: data}
}

func Paginated(data interface{}, meta Meta) Envelope {
	return Envelope{Success: true, Data: data, Meta: &meta}
}

func Err(err *apperrors.AppError) Envelope {
	return Envelope{
		Success: false,
		Error:   &ErrBody{Code: err.Code, Message: err.Message},
	}
}

func ErrWithFields(err *apperrors.AppError, fields map[string]string) Envelope {
	return Envelope{
		Success: false,
		Error:   &ErrBody{Code: err.Code, Message: err.Message, Fields: fields},
	}
}
