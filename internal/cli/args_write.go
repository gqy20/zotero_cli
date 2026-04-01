package cli

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func parseWriteCreateArgs(args []string) (map[string]any, int, bool, error) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--data")
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--from-file")
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return nil, 0, false, errInvalidFlagValue("--if-unmodified-since-version")
			}
			version = parsed
			versionSet = true
		default:
			return nil, 0, false, errUnexpectedArgument(args[i])
		}
	}

	if raw != "" && fromFile != "" {
		return nil, 0, false, errConflictingDataSources()
	}
	if raw == "" && fromFile == "" {
		return nil, 0, false, errMissingWritePayload()
	}
	if !versionSet {
		return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, 0, false, errReadFromFile(err)
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, 0, false, errInvalidJSONPayload(err)
	}
	return data, version, jsonOutput, nil
}

func parseWriteUpdateArgs(args []string, requireVersion bool) (string, map[string]any, int, bool, error) {
	if len(args) == 0 {
		return "", nil, 0, false, errMissingItemKey()
	}
	key := args[0]
	data, version, jsonOutput, err := parseWriteCreateArgs(args[1:])
	if err == nil {
		return key, data, version, jsonOutput, nil
	}

	data, version, jsonOutput, err = parseWriteCreateLikeArgs(args[1:], requireVersion)
	if err != nil {
		return "", nil, 0, false, err
	}
	return key, data, version, jsonOutput, nil
}

func parseWriteDeleteArgs(args []string) (string, int, bool, error) {
	if len(args) == 0 {
		return "", 0, false, errMissingItemKey()
	}
	key := args[0]
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				return "", 0, false, errMissingFlagValue("--if-unmodified-since-version")
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return "", 0, false, errInvalidFlagValue("--if-unmodified-since-version")
			}
			version = parsed
			versionSet = true
		default:
			return "", 0, false, errUnexpectedArgument(args[i])
		}
	}

	if !versionSet {
		return "", 0, false, errMissingFlagValue("--if-unmodified-since-version")
	}

	return key, version, jsonOutput, nil
}

func parseWriteCreateLikeArgs(args []string, requireVersion bool) (map[string]any, int, bool, error) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--data")
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--from-file")
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return nil, 0, false, errInvalidFlagValue("--if-unmodified-since-version")
			}
			version = parsed
			versionSet = true
		default:
			return nil, 0, false, errUnexpectedArgument(args[i])
		}
	}

	if raw != "" && fromFile != "" {
		return nil, 0, false, errConflictingDataSources()
	}
	if raw == "" && fromFile == "" {
		return nil, 0, false, errMissingWritePayload()
	}
	if requireVersion && !versionSet {
		return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, 0, false, errReadFromFile(err)
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, 0, false, errInvalidJSONPayload(err)
	}
	return data, version, jsonOutput, nil
}

func parseWriteBatchArgs(args []string, requireVersion bool) ([]map[string]any, int, bool, error) {
	var raw string
	var fromFile string
	var version int
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--data":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--data")
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--from-file")
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return nil, 0, false, errInvalidFlagValue("--if-unmodified-since-version")
			}
			version = parsed
			versionSet = true
		default:
			return nil, 0, false, errUnexpectedArgument(args[i])
		}
	}

	if raw != "" && fromFile != "" {
		return nil, 0, false, errConflictingDataSources()
	}
	if raw == "" && fromFile == "" {
		return nil, 0, false, errMissingWritePayload()
	}
	if requireVersion && !versionSet {
		return nil, 0, false, errMissingFlagValue("--if-unmodified-since-version")
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			return nil, 0, false, errReadFromFile(err)
		}
		raw = string(content)
	}

	var data []map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil || len(data) == 0 {
		if err != nil {
			return nil, 0, false, errInvalidJSONPayload(err)
		}
		return nil, 0, false, errEmptyBatchPayload()
	}
	return data, version, jsonOutput, nil
}

func parseKeysListArgs(args []string, requireVersion bool, requireTag bool) ([]string, int, string, bool, error) {
	var keysValue string
	var version int
	var tag string
	var jsonOutput bool
	var versionSet bool

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--items":
			if i+1 >= len(args) {
				return nil, 0, "", false, errMissingFlagValue("--items")
			}
			i++
			keysValue = args[i]
		case "--tag":
			if i+1 >= len(args) {
				return nil, 0, "", false, errMissingFlagValue("--tag")
			}
			i++
			tag = strings.TrimSpace(args[i])
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				return nil, 0, "", false, errMissingFlagValue("--if-unmodified-since-version")
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				return nil, 0, "", false, errInvalidFlagValue("--if-unmodified-since-version")
			}
			version = parsed
			versionSet = true
		default:
			return nil, 0, "", false, errUnexpectedArgument(args[i])
		}
	}

	keys := make([]string, 0)
	for _, key := range strings.Split(keysValue, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 {
		return nil, 0, "", false, errMissingFlagValue("--items")
	}
	if requireVersion && !versionSet {
		return nil, 0, "", false, errMissingFlagValue("--if-unmodified-since-version")
	}
	if requireTag && tag == "" {
		return nil, 0, "", false, errMissingFlagValue("--tag")
	}

	return keys, version, tag, jsonOutput, nil
}
