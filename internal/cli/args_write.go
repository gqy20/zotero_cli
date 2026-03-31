package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func parseWriteCreateArgs(args []string, usage string) (map[string]any, int, bool, bool) {
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
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || !versionSet {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func parseWriteUpdateArgs(args []string, usage string, requireVersion bool) (string, map[string]any, int, bool, bool) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return "", nil, 0, false, false
	}
	key := args[0]
	data, version, jsonOutput, ok := parseWriteCreateArgs(args[1:], usage)
	if ok {
		return key, data, version, jsonOutput, true
	}

	data, version, jsonOutput, ok = parseWriteCreateLikeArgs(args[1:], usage, requireVersion)
	if !ok {
		return "", nil, 0, false, false
	}
	return key, data, version, jsonOutput, true
}

func parseWriteDeleteArgs(args []string, usage string) (string, int, bool, bool) {
	if len(args) == 0 {
		fmt.Fprintln(stderr, usage)
		return "", 0, false, false
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
				fmt.Fprintln(stderr, usage)
				return "", 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return "", 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return "", 0, false, false
		}
	}

	if !versionSet {
		fmt.Fprintln(stderr, usage)
		return "", 0, false, false
	}

	return key, version, jsonOutput, true
}

func parseWriteCreateLikeArgs(args []string, usage string, requireVersion bool) (map[string]any, int, bool, bool) {
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
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || (requireVersion && !versionSet) {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func parseWriteBatchArgs(args []string, usage string, requireVersion bool) ([]map[string]any, int, bool, bool) {
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
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			raw = args[i]
		case "--from-file":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			fromFile = args[i]
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
	}

	if (raw == "" && fromFile == "") || (raw != "" && fromFile != "") || (requireVersion && !versionSet) {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}

	if fromFile != "" {
		content, err := os.ReadFile(fromFile)
		if err != nil {
			fmt.Fprintln(stderr, usage)
			return nil, 0, false, false
		}
		raw = string(content)
	}

	var data []map[string]any
	if err := json.Unmarshal([]byte(raw), &data); err != nil || len(data) == 0 {
		fmt.Fprintln(stderr, usage)
		return nil, 0, false, false
	}
	return data, version, jsonOutput, true
}

func parseKeysListArgs(args []string, usage string, requireVersion bool, requireTag bool) ([]string, int, string, bool, bool) {
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
				fmt.Fprintln(stderr, usage)
				return nil, 0, "", false, false
			}
			i++
			keysValue = args[i]
		case "--tag":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, "", false, false
			}
			i++
			tag = strings.TrimSpace(args[i])
		case "--if-unmodified-since-version":
			if i+1 >= len(args) {
				fmt.Fprintln(stderr, usage)
				return nil, 0, "", false, false
			}
			i++
			parsed, err := strconv.Atoi(args[i])
			if err != nil || parsed <= 0 {
				fmt.Fprintln(stderr, usage)
				return nil, 0, "", false, false
			}
			version = parsed
			versionSet = true
		default:
			fmt.Fprintln(stderr, usage)
			return nil, 0, "", false, false
		}
	}

	keys := make([]string, 0)
	for _, key := range strings.Split(keysValue, ",") {
		key = strings.TrimSpace(key)
		if key != "" {
			keys = append(keys, key)
		}
	}

	if len(keys) == 0 || (requireVersion && !versionSet) || (requireTag && tag == "") {
		fmt.Fprintln(stderr, usage)
		return nil, 0, "", false, false
	}

	return keys, version, tag, jsonOutput, true
}
