package cli

import (
	"fmt"
	"strconv"
)

func (c *CLI) parseJSONOnlyArgs(args []string, usage string) (bool, bool, bool) {
	jsonOutput := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--help" || arg == "-h" {
			fmt.Fprintln(c.stdout, usage)
			return false, true, true
		}
		fmt.Fprintln(c.stderr, usage)
		return false, false, false
	}
	return jsonOutput, true, false
}

func (c *CLI) parseJSONAndLimitArgs(args []string, usage string) (bool, int, bool, bool) {
	jsonOutput := false
	limit := 0

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--limit":
			if i+1 >= len(args) {
				fmt.Fprintln(c.stderr, "error: missing value for --limit")
				fmt.Fprintln(c.stderr, usage)
				return false, 0, false, false
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 {
				fmt.Fprintln(c.stderr, "error: invalid value for --limit")
				fmt.Fprintln(c.stderr, usage)
				return false, 0, false, false
			}
			limit = n
			i++
		case "--help", "-h":
			fmt.Fprintln(c.stdout, usage)
			return false, 0, true, true
		default:
			fmt.Fprintln(c.stderr, usage)
			return false, 0, false, false
		}
	}
	return jsonOutput, limit, true, false
}

func (c *CLI) parseSingleValueCommand(args []string, usage string) (string, bool, bool, bool) {
	jsonOutput := false
	value := ""

	for _, arg := range args {
		if arg == "--json" {
			jsonOutput = true
			continue
		}
		if arg == "--help" || arg == "-h" {
			fmt.Fprintln(c.stdout, usage)
			return "", false, true, true
		}
		if value == "" {
			value = arg
			continue
		}
		fmt.Fprintln(c.stderr, usage)
		return "", false, false, false
	}

	return value, jsonOutput, true, false
}
