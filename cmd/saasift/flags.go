package main

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

type headerFlags http.Header

func (h headerFlags) String() string {
	var parts []string
	for name, values := range h {
		for _, v := range values {
			parts = append(parts, name+": "+v)
		}
	}
	return strings.Join(parts, ", ")
}

func (h headerFlags) Set(value string) error {
	name, val, found := strings.Cut(value, ":")
	name = strings.TrimSpace(name)
	if !found || name == "" {
		return fmt.Errorf("header must be in format 'Name: value', got %q", value)
	}
	http.Header(h).Add(name, strings.TrimSpace(val))
	return nil
}

type intSet map[int]bool

func (s intSet) String() string {
	nums := make([]int, 0, len(s))

	for n := range s {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	parts := make([]string, len(nums))
	for i, n := range nums {
		parts[i] = strconv.Itoa((n))
	}
	return strings.Join(parts, ",")
}

func (s intSet) Set(value string) error {
	clear(s)
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil {
			return fmt.Errorf("%q is not a number", part)
		}
		s[n] = true
	}
	return nil
}
