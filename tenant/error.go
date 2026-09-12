package tenant

// HTTPError is a stable hosting error surfaced by both API families.
type HTTPError struct {
	Code    string
	Message string
	Status  int
}

func (e *HTTPError) Error() string { return e.Message }

func (e *HTTPError) Is(target error) bool {
	other, ok := target.(*HTTPError)
	if !ok || e == nil || other == nil {
		return false
	}
	return e.Code == other.Code && e.Message == other.Message
}
