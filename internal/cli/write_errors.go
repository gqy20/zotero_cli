package cli

import (
	"errors"
	"fmt"
)

var (
	errWritePayloadMissing = errors.New("missing write payload")
	errDataSourcesConflict = errors.New("conflicting write payload sources")
	errMissingItemKeyValue = errors.New("missing item key")
)

func errMissingFlagValue(flag string) error {
	return fmt.Errorf("missing value for %s", flag)
}

func errInvalidFlagValue(flag string) error {
	return fmt.Errorf("invalid value for %s", flag)
}

func errUnexpectedArgument(arg string) error {
	return fmt.Errorf("unexpected argument: %s", arg)
}

func errConflictingDataSources() error {
	return fmt.Errorf("cannot combine --data and --from-file")
}

func errMissingWritePayload() error {
	return errWritePayloadMissing
}

func errReadFromFile(err error) error {
	return fmt.Errorf("could not read --from-file: %w", err)
}

func errInvalidJSONPayload(err error) error {
	return fmt.Errorf("invalid JSON payload: %w", err)
}

func errMissingItemKey() error {
	return errMissingItemKeyValue
}
