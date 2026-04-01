package backend

import "fmt"

var (
	ErrItemNotFound       = fmt.Errorf("item not found")
	ErrUnsupportedFeature = fmt.Errorf("unsupported backend feature")
)

type itemNotFoundError struct {
	Object string
	Key    string
}

func newItemNotFoundError(object string, key string) error {
	return &itemNotFoundError{Object: object, Key: key}
}

func (e *itemNotFoundError) Error() string {
	if e.Object == "" {
		return fmt.Sprintf("item not found: %s", e.Key)
	}
	return fmt.Sprintf("%s not found: %s", e.Object, e.Key)
}

func (e *itemNotFoundError) Unwrap() error {
	return ErrItemNotFound
}

type unsupportedFeatureError struct {
	Backend string
	Feature string
}

func newUnsupportedFeatureError(backend string, feature string) error {
	return &unsupportedFeatureError{Backend: backend, Feature: feature}
}

func (e *unsupportedFeatureError) Error() string {
	if e.Backend == "" {
		return fmt.Sprintf("unsupported feature: %s", e.Feature)
	}
	return fmt.Sprintf("%s does not support %s", e.Backend, e.Feature)
}

func (e *unsupportedFeatureError) Unwrap() error {
	return ErrUnsupportedFeature
}
