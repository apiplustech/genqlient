package graphql

type Option func(r *Request)

// This option will set the request to do file upload.
// If the method of http client is GET, this will be invalid.
func MultipartUploadOption() Option {
	return func(r *Request) {
		r.UploadFile = true
	}
}
