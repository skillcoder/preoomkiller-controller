package k8s

// TooManyRequestsError represents a "too many requests" case that is not an error.
type TooManyRequestsError struct{}

func (e *TooManyRequestsError) Error() string {
	return "too many requests"
}

func (e *TooManyRequestsError) IsTooManyRequests() {}

var errTooManyRequests = &TooManyRequestsError{}

// PodNotFoundError represents a "not found" case that is not an error.
type PodNotFoundError struct{}

func (e *PodNotFoundError) Error() string {
	return "pod not found"
}

func (e *PodNotFoundError) IsNotFound() {}

var errPodNotFound = &PodNotFoundError{}
