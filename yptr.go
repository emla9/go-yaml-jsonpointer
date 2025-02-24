// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: BSD-2-Clause

package yptr

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/zillow/go-yaml/v3"
)

const (
	emptyJSONPointer     = ``
	jsonPointerSeparator = `/`
)

var (
	// ErrTooManyResults means a pointer matches too many results (usually more than one expected result).
	ErrTooManyResults = fmt.Errorf("too many results")
	// ErrNotFound a pointer failed to find a match.
	ErrNotFound = fmt.Errorf("not found")
)

// FindAll finds all locations in the json/yaml tree pointed by root that match the extended
// JSONPointer passed in ptr.
func FindAll(root *yaml.Node, ptr string) ([]*yaml.Node, error) {
	if ptr == "" {
		return nil, fmt.Errorf("invalid empty pointer")
	}

	toks, err := jsonPointerToTokens(ptr)
	if err != nil {
		return nil, err
	}

	res, err := find(root, toks)
	if err != nil {
		return nil, fmt.Errorf("%q: %w", ptr, err)
	}
	return res, nil
}

// Find is like FindAll but returns ErrTooManyResults if multiple matches are located.
func Find(root *yaml.Node, ptr string) (*yaml.Node, error) {
	res, err := FindAll(root, ptr)
	if err != nil {
		return nil, err
	}
	if len(res) > 1 {
		return nil, fmt.Errorf("got %d matches: %w", len(res), ErrTooManyResults)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("bad state while finding %q: res is empty but error is: %v", ptr, err)
	}
	return res[0], nil
}

// find recursively matches a token against a yaml node.
func find(root *yaml.Node, toks []string) ([]*yaml.Node, error) {
	next, err := match(root, toks[0])
	if err != nil {
		return nil, err
	}
	if len(toks) == 1 {
		return next, nil
	}

	var res []*yaml.Node
	for _, n := range next {
		f, err := find(n, toks[1:])
		if err != nil {
			return nil, err
		}
		res = append(res, f...)
	}
	return res, nil
}

// match matches a JSONPointer token against a yaml Node.
//
// If root is a map, it performs a field lookup using tok as field name,
// and if found it will return a singleton slice containing the value contained
// in that field.
//
// If root is an array and tok is a number i, it will return the ith element of that array.
// If tok is ~{...}, it will parse the {...} object as a JSON object
// and use it to filter the array using a treeSubsetPred.
// If tok is ~[key=value] it will use keyValuePred to filter the array.
func match(root *yaml.Node, tok string) ([]*yaml.Node, error) {
	c := root.Content
	switch root.Kind {
	case yaml.MappingNode:
		if l := len(c); l%2 != 0 {
			return nil, fmt.Errorf("yaml.Node invariant broken, found %d map content", l)
		}

		for i := 0; i < len(c); i += 2 {
			key, value := c[i], c[i+1]
			if tok == key.Value {
				return []*yaml.Node{value}, nil
			}
		}
	case yaml.SequenceNode:
		switch {
		case strings.HasPrefix(tok, "~{"): // subtree match: ~{"name":"app"}
			var mtree yaml.Node
			if err := yaml.Unmarshal([]byte(tok[1:]), &mtree); err != nil {
				return nil, err
			}
			return filter(c, treeSubsetPred(&mtree))
		default:
			i, err := strconv.Atoi(tok)
			if err != nil {
				return nil, err
			}
			if i < 0 || i >= len(c) {
				return nil, fmt.Errorf("out of bounds")
			}
			return c[i : i+1], nil
		}
	case yaml.DocumentNode:
		// skip document nodes.
		return match(c[0], tok)
	default:
		return nil, fmt.Errorf("unhandled node type: %v (%v)", root.Kind, root.Tag)
	}
	return nil, fmt.Errorf("%q: %w", tok, ErrNotFound)
}

type nodePredicate func(*yaml.Node) bool

// filter applies a nodePredicate to each input node and returns only those for which the predicate
// function returns true.
func filter(nodes []*yaml.Node, pred nodePredicate) ([]*yaml.Node, error) {
	var res []*yaml.Node
	for _, n := range nodes {
		if pred(n) {
			res = append(res, n)
		}
	}
	return res, nil
}

// A treeSubsetPred is a node predicate that returns true if tree a is a subset of tree b.
func treeSubsetPred(a *yaml.Node) nodePredicate {
	return func(b *yaml.Node) bool {
		return isTreeSubset(a, b)
	}
}

// Given a JSON pointer, return its tokens or an error if invalid.
func jsonPointerToTokens(jsonPointer string) ([]string, error) {
	if jsonPointer == emptyJSONPointer {
		return []string{}, nil
	}

	if err := ValidateJSONPointer(jsonPointer); err != nil {
		return nil, err
	}

	referenceTokens := strings.Split(jsonPointer, jsonPointerSeparator)
	return referenceTokens[1:], nil
}

// Given a JSON pointer, return an error if invalid.
func ValidateJSONPointer(jsonPointer string) error {
	if jsonPointer == emptyJSONPointer {
		return nil
	}

	if !strings.HasPrefix(jsonPointer, jsonPointerSeparator) {
		return fmt.Errorf(`JSON pointer must be empty or start with a "` + jsonPointerSeparator)
	}

	return nil
}
