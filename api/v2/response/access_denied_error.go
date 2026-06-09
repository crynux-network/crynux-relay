package response

type AccessDeniedErrorResponse struct {
	ErrorResponse
}

func NewAccessDeniedErrorResponse() *AccessDeniedErrorResponse {
	r := &AccessDeniedErrorResponse{}
	r.SetErrorType("Access denied")
	return r
}
