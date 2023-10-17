package utils

import (
	"fmt"
	"strings"
)

//Handling Bool flags with an explicit type

type BoolStr bool

func (b *BoolStr) Set(s string) error {
	s = strings.ToLower(s)
	t := BoolStr(s == "true" || s == "1" || s == "t" || s == "y" || s == "yes")
	if !t {
		f := BoolStr(s == "false" || s == "0" || s == "f" || s == "n" || s == "no")
		if !f { // value is neither true nor false.
			return fmt.Errorf("invalid boolean value: %q (valid values: true, false)", s)
		}
	}
	*b = t
	return nil
}

func (b *BoolStr) Type() string {
	return "boolean"
}

func (b *BoolStr) String() string {
	if *b {
		return "true"
	}
	return "false"
}
