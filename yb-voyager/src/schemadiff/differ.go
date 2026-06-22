// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemadiff

import "github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"

// Config configures a Differ. The zero value applies no filtering (pass-through).
//
// WHY a struct instead of functional options: the number of knobs is expected
// to remain small (currently one; an IgnoreRules field will land next). A plain
// struct is easier to read, copy, and compare than a slice of option functions,
// and the zero value gives free pass-through semantics at no cost to callers.
//
// NOTE: an IgnoreRules field will be added here when ignore-rule support lands;
// adding a field is non-breaking.
type Config struct {
	Scope Scope // post-diff table/object-type scope; zero value keeps everything
}

// Differ is the configured entry point into the schemadiff package: it runs
// Diff and then applies the configured post-diff filters.
//
// WHY a façade: callers that need consistent filtering (scope, and eventually
// ignore-rules) should not have to chain Diff → FilterByScope themselves and
// risk forgetting a step. Differ encapsulates that pipeline behind a single
// call. The package-level Diff and FilterByScope remain the underlying pure
// mechanism and are available to callers who want the raw, unfiltered diff.
type Differ struct {
	cfg Config
}

// NewDiffer constructs a Differ with the given Config. The returned value is
// always non-nil. Passing a zero Config creates a pass-through Differ whose
// Diff output is identical to the package-level Diff function.
func NewDiffer(cfg Config) *Differ {
	return &Differ{cfg: cfg}
}

// Diff computes the differences between snapshots a and b and applies the
// configured filters (currently scope). With a zero Config it is equivalent to
// the package-level Diff.
//
// Note: the method (d *Differ) Diff and the package function Diff coexisting is
// intentional and valid Go — callers in possession of a *Differ call the method;
// callers that want an unfiltered raw diff call the function.
func (d *Differ) Diff(a, b *schemasnapshot.SchemaSnapshot) []Difference {
	return FilterByScope(Diff(a, b), d.cfg.Scope)
}
