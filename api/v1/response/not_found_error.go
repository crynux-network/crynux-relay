package response

type NotFoundErrorResponse struct {
	ErrorResponse
}

func NewNotFoundErrorResponse() *NotFoundErrorResponse {
	r := &NotFoundErrorResponse{}
	r.SetErrorType("Not found")
	return r
}