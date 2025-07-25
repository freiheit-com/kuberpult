package errorMatcher

// For testing purposes only

type ErrMatcher struct {
	Message string
}

func (e ErrMatcher) Error() string {
	return e.Message
}

func (e ErrMatcher) Is(err error) bool {
	return e.Error() == err.Error()
}
