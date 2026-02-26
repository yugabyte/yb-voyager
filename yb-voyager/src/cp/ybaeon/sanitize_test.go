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
package ybaeon

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizePayload_Nil(t *testing.T) {
	assert.Nil(t, sanitizePayload(nil))
}

func TestSanitizePayload_PlainString(t *testing.T) {
	result := sanitizePayload("no html here")
	assert.Equal(t, "no html here", result)
}

func TestSanitizePayload_StringWithAnchorTag(t *testing.T) {
	input := `Check <a href="https://docs.yugabyte.com">docs</a> for details`
	expected := `Check docs (https://docs.yugabyte.com) for details`
	result := sanitizePayload(input)
	assert.Equal(t, expected, result)
}

func TestSanitizePayload_StringWithAnchorTagSameTextAndURL(t *testing.T) {
	input := `<a href="https://example.com">https://example.com</a>`
	expected := `https://example.com`
	result := sanitizePayload(input)
	assert.Equal(t, expected, result)
}

func TestSanitizePayload_SimpleStruct(t *testing.T) {
	type SimplePayload struct {
		Name    string
		Count   int
		Details string
	}
	input := SimplePayload{
		Name:    `<a href="https://example.com">Example</a>`,
		Count:   42,
		Details: "plain text",
	}
	result := sanitizePayload(input).(SimplePayload)
	assert.Equal(t, "Example (https://example.com)", result.Name)
	assert.Equal(t, 42, result.Count)
	assert.Equal(t, "plain text", result.Details)
}

func TestSanitizePayload_NestedStruct(t *testing.T) {
	type Inner struct {
		Link string
	}
	type Outer struct {
		Label string
		Inner Inner
	}
	input := Outer{
		Label: "clean",
		Inner: Inner{
			Link: `<a href="https://docs.yugabyte.com">YB Docs</a>`,
		},
	}
	result := sanitizePayload(input).(Outer)
	assert.Equal(t, "clean", result.Label)
	assert.Equal(t, "YB Docs (https://docs.yugabyte.com)", result.Inner.Link)
}

func TestSanitizePayload_SliceOfStrings(t *testing.T) {
	input := []string{
		`<a href="https://example.com">Link1</a>`,
		"plain text",
		`<a href="https://example2.com">https://example2.com</a>`,
	}
	result := sanitizePayload(input).([]string)
	assert.Equal(t, "Link1 (https://example.com)", result[0])
	assert.Equal(t, "plain text", result[1])
	assert.Equal(t, "https://example2.com", result[2])
}

func TestSanitizePayload_StructWithSlice(t *testing.T) {
	type Payload struct {
		Notes []string
		Count int
	}
	input := Payload{
		Notes: []string{
			`Refer to <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/releases/">release notes</a> for details.`,
			"No HTML here.",
		},
		Count: 5,
	}
	result := sanitizePayload(input).(Payload)
	assert.Equal(t, "Refer to release notes (https://docs.yugabyte.com/preview/releases/) for details.", result.Notes[0])
	assert.Equal(t, "No HTML here.", result.Notes[1])
	assert.Equal(t, 5, result.Count)
}

func TestSanitizePayload_MapStringInterface(t *testing.T) {
	input := map[string]interface{}{
		"link":  `<a href="https://example.com">click</a>`,
		"count": 10,
		"plain": "no html",
	}
	result := sanitizePayload(input).(map[string]interface{})
	assert.Equal(t, "click (https://example.com)", result["link"])
	assert.Equal(t, 10, result["count"])
	assert.Equal(t, "no html", result["plain"])
}

func TestSanitizePayload_NilSlice(t *testing.T) {
	type Payload struct {
		Items []string
	}
	input := Payload{Items: nil}
	result := sanitizePayload(input).(Payload)
	assert.Nil(t, result.Items)
}

func TestSanitizePayload_EmptyMap(t *testing.T) {
	input := map[string]interface{}{}
	result := sanitizePayload(input).(map[string]interface{})
	assert.Empty(t, result)
}

func TestSanitizePayload_PointerField(t *testing.T) {
	type Inner struct {
		Link string
	}
	type Outer struct {
		Inner *Inner
	}
	inner := &Inner{Link: `<a href="https://example.com">Example</a>`}
	input := Outer{Inner: inner}
	result := sanitizePayload(input).(Outer)
	assert.Equal(t, "Example (https://example.com)", result.Inner.Link)
}

func TestSanitizePayload_NilPointer(t *testing.T) {
	type Inner struct {
		Link string
	}
	type Outer struct {
		Inner *Inner
	}
	input := Outer{Inner: nil}
	result := sanitizePayload(input).(Outer)
	assert.Nil(t, result.Inner)
}

func TestSanitizePayload_RealisticNotes(t *testing.T) {
	// Mirrors the actual NoteInfo.Text values from assessMigrationCommand.go
	type AssessmentPayload struct {
		GeneralNotes         []string
		ColocatedShardedNotes []string
		SizingNotes          []string
		VoyagerVersion       string
	}
	input := AssessmentPayload{
		GeneralNotes: []string{
			`Some features listed in this report may be supported under a preview flag in the specified target-db-version of YugabyteDB. Please refer to the official <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/releases/ybdb-releases/">release notes</a> for detailed information and usage guidelines.`,
			`<a class="highlight-link" target="_blank"  href="https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/#assessment-and-schema-analysis-limitations">Limitations in assessment</a>`,
		},
		ColocatedShardedNotes: []string{
			`For additional considerations related to colocated tables, refer to the documentation at: <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations">https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations</a>`,
		},
		SizingNotes:    []string{"No HTML here."},
		VoyagerVersion: "2026.2.2",
	}
	result := sanitizePayload(input).(AssessmentPayload)

	assert.Equal(t,
		`Some features listed in this report may be supported under a preview flag in the specified target-db-version of YugabyteDB. Please refer to the official release notes (https://docs.yugabyte.com/preview/releases/ybdb-releases/) for detailed information and usage guidelines.`,
		result.GeneralNotes[0])
	assert.Equal(t,
		`Limitations in assessment (https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/#assessment-and-schema-analysis-limitations)`,
		result.GeneralNotes[1])
	assert.Equal(t,
		`For additional considerations related to colocated tables, refer to the documentation at: https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations`,
		result.ColocatedShardedNotes[0])
	assert.Equal(t, "No HTML here.", result.SizingNotes[0])
	assert.Equal(t, "2026.2.2", result.VoyagerVersion)
}

func TestSanitizePayload_NonAnchorHTMLTagsPassThrough(t *testing.T) {
	type Payload struct {
		Text string
	}
	// StripAnchorTags only handles <a> tags; other HTML tags are left as-is
	input := Payload{Text: `<div>content</div> and <span>more</span> and <br/>`}
	result := sanitizePayload(input).(Payload)
	assert.Equal(t, `<div>content</div> and <span>more</span> and <br/>`, result.Text)
}

func TestSanitizePayload_MixedAnchorAndOtherTags(t *testing.T) {
	type Payload struct {
		Text string
	}
	input := Payload{Text: `<div><a href="https://example.com">link</a> and <span>text</span></div>`}
	result := sanitizePayload(input).(Payload)
	assert.Equal(t, `<div>link (https://example.com) and <span>text</span></div>`, result.Text)
}

func TestSanitizePayload_DoesNotModifyOriginal(t *testing.T) {
	type Payload struct {
		Text string
	}
	original := Payload{Text: `<a href="https://example.com">link</a>`}
	_ = sanitizePayload(original)
	assert.Equal(t, `<a href="https://example.com">link</a>`, original.Text)
}
