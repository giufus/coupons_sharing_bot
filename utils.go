package main

import (
	"fmt"
	"strconv"
	"strings"
)

func parseKeyValues(input string) (map[string]string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return map[string]string{}, nil
	}

	res := map[string]string{}
	for i := 0; i < len(input); {
		for i < len(input) && input[i] == ' ' {
			i++
		}
		if i >= len(input) {
			break
		}

		startKey := i
		for i < len(input) && input[i] != '=' && input[i] != ' ' {
			i++
		}
		if i >= len(input) || input[i] != '=' {
			return nil, fmt.Errorf("missing '=' after key")
		}
		key := strings.ToLower(strings.TrimSpace(input[startKey:i]))
		i++ // skip '='

		var val string
		if i < len(input) && input[i] == '"' {
			i++
			startVal := i
			for i < len(input) && input[i] != '"' {
				i++
			}
			if i >= len(input) {
				return nil, fmt.Errorf("unterminated quote")
			}
			val = input[startVal:i]
			i++ // skip closing quote
		} else {
			startVal := i
			for i < len(input) && input[i] != ' ' {
				i++
			}
			val = strings.TrimSpace(input[startVal:i])
		}

		if key == "" {
			return nil, fmt.Errorf("missing key")
		}
		res[key] = val
	}
	return res, nil
}

func parseOptionalInt(v string) (*int, error) {
	if v == "" {
		return nil, nil
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return nil, err
	}
	return &i, nil
}

func parseInt(v string) (int, error) {
	return strconv.Atoi(v)
}

func parseTags(v string) []string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	var tags []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			tags = append(tags, p)
		}
	}
	return tags
}

func formatMonth(m *int) string {
	if m == nil {
		return "-"
	}
	return fmt.Sprintf("%02d", *m)
}
