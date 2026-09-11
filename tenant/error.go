package tenant

// HTTPError is a stable hosting error surfaced by both API families.
type HTTPError struct {
	Code    string
	Message string
	Status  int
}

func (e *HTTPError) Error() string { return e.Message }
