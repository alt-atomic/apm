package classify

type APMError struct {
	Type string
	Err  error
}

func (e APMError) Error() string {
	return e.Err.Error()
}

func New(errorType string, err error) APMError {
	return APMError{Type: errorType, Err: err}
}

const (
	ErrorTypeDatabase   = "database"
	ErrorTypeRepository = "repository"
	ErrorTypeContainer  = "container"
	ErrorTypeApt        = "apt"
)
