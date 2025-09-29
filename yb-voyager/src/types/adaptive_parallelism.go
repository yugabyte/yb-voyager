/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package types

import (
	"fmt"
	"strings"
)

/*
AdaptiveParallelismMode defines the mode for adaptive parallelism behavior:
- disabled: Adaptive parallelism is turned off
- balanced: Adaptive parallelism operates with moderate thresholds
- aggressive: Adaptive parallelism operates with aggressive thresholds for maximum performance
*/
type AdaptiveParallelismMode string

const (
	// Mode values
	DisabledAdaptiveParallelismModeName   = "disabled"
	BalancedAdaptiveParallelismModeName   = "balanced"
	AggressiveAdaptiveParallelismModeName = "aggressive"

	// Mode consts
	DisabledAdaptiveParallelismMode   AdaptiveParallelismMode = DisabledAdaptiveParallelismModeName
	BalancedAdaptiveParallelismMode   AdaptiveParallelismMode = BalancedAdaptiveParallelismModeName
	AggressiveAdaptiveParallelismMode AdaptiveParallelismMode = AggressiveAdaptiveParallelismModeName
)

var validAdaptiveParallelismModeNames = []string{
	DisabledAdaptiveParallelismModeName,
	BalancedAdaptiveParallelismModeName,
	AggressiveAdaptiveParallelismModeName,
}

func (a *AdaptiveParallelismMode) Type() string {
	return "AdaptiveParallelismMode"
}

func (a *AdaptiveParallelismMode) String() string {
	return string(*a)
}

func (a *AdaptiveParallelismMode) Set(s string) error {
	mode, err := NewAdaptiveParallelismMode(s)
	if err != nil {
		return err
	}
	*a = mode
	return nil
}

func (a *AdaptiveParallelismMode) IsEnabled() bool {
	return *a != DisabledAdaptiveParallelismMode
}

func NewAdaptiveParallelismMode(s string) (AdaptiveParallelismMode, error) {
	s = strings.ToLower(s)
	switch s {
	case DisabledAdaptiveParallelismModeName:
		return DisabledAdaptiveParallelismMode, nil
	case BalancedAdaptiveParallelismModeName:
		return BalancedAdaptiveParallelismMode, nil
	case AggressiveAdaptiveParallelismModeName:
		return AggressiveAdaptiveParallelismMode, nil
	default:
		return "", fmt.Errorf("invalid adaptive parallelism mode: %q. Allowed modes: %v", s, validAdaptiveParallelismModeNames)
	}
}
