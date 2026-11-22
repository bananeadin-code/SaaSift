package main

import (
	"fmt"
	"net/http"
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
