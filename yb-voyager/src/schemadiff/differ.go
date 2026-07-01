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
type Config struct {
	Scope Scope // post-diff table/object-type scope; zero value keeps everything
}

// Differ is the configured entry point: it runs Diff and applies the configured
// post-diff filters. The package-level Diff and FilterByScope remain available to
// callers who want the raw, unfiltered diff.
type Differ struct {
	cfg Config
}

// NewDiffer constructs a Differ with the given Config. A zero Config yields a
// pass-through Differ whose Diff output matches the package-level Diff function.
func NewDiffer(cfg Config) *Differ {
	return &Differ{cfg: cfg}
}

// Diff computes the differences between snapshots a and b and applies the
// configured filters. With a zero Config it is equivalent to the package-level Diff.
func (d *Differ) Diff(a, b *schemasnapshot.SnapshotContent) []Difference {
	return FilterByScope(Diff(a, b), d.cfg.Scope)
}
