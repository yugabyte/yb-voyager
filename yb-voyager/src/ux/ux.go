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

package ux

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

var (
	BannerColor    = color.New(color.FgCyan, color.Bold)
	StageColor     = color.New(color.FgCyan)
	CheckColor     = color.New(color.FgGreen)
	SkipColor      = color.New(color.FgYellow)
	FailColor      = color.New(color.FgRed)
	SeparatorColor = color.New(color.FgHiBlack)
	DimColor       = color.New(color.FgHiBlack)
	BoldColor      = color.New(color.Bold)
	PathColor      = color.New(color.Underline)
)

const defaultBoxWidth = 60

// BannerRow represents a key-value row inside the banner box.
type BannerRow struct {
	Key   string
	Value string
}

// PrintBanner renders a box-drawn banner with a title and optional detail rows.
//
//	┌──────────────────────────────────────────────────────────┐
//	│  YugabyteDB Voyager — Migration Assessment              │
//	├──────────────────────────────────────────────────────────┤
//	│  Voyager version   main                                 │
//	│  Target DB         YugabyteDB 2025.2.2.1                │
//	│  Source             postgresql (localhost:5432/mydb)     │
//	│  Export directory   /path/to/export                     │
//	└──────────────────────────────────────────────────────────┘
func PrintBanner(title string, rows []BannerRow) {
	keyWidth := 0
	for _, r := range rows {
		if len(r.Key) > keyWidth {
			keyWidth = len(r.Key)
		}
	}

	// "│  " + key (padded to keyWidth) + "  " + value + " │"  =  keyWidth + len(value) + 7
	width := defaultBoxWidth
	titleLen := len(title) + 4
	for _, r := range rows {
		rowLen := keyWidth + len(r.Value) + 7
		if rowLen > width {
			width = rowLen
		}
	}
	if titleLen > width {
		width = titleLen
	}
	inner := width - 2

	top := "┌" + strings.Repeat("─", inner) + "┐"
	mid := "├" + strings.Repeat("─", inner) + "┤"
	bot := "└" + strings.Repeat("─", inner) + "┘"

	BannerColor.Println(top)
	BannerColor.Printf("│  %-*s│\n", inner-2, title)

	if len(rows) > 0 {
		BannerColor.Println(mid)

		for _, r := range rows {
			valWidth := inner - keyWidth - 5
			if valWidth < 0 {
				valWidth = 0
			}
			BannerColor.Print("│  ")
			DimColor.Printf("%-*s", keyWidth, r.Key)
			BannerColor.Print("  ")
			fmt.Printf("%-*s", valWidth, r.Value)
			BannerColor.Println(" │")
		}
	}

	BannerColor.Println(bot)
	fmt.Println()
}

// PrintSeparator prints a dim horizontal rule.
func PrintSeparator() {
	SeparatorColor.Println(strings.Repeat("─", defaultBoxWidth))
}

// PrintSectionHeader prints a bold section header.
func PrintSectionHeader(label string) {
	BoldColor.Println(label)
}

// PrintPreflightCheck prints a green checkmark with the given label.
func PrintPreflightCheck(label string) {
	CheckColor.Printf("  ✓ %s\n", label)
}

// PrintPreflightSkip prints a yellow dash with the given label.
func PrintPreflightSkip(label string) {
	SkipColor.Printf("  - %s\n", label)
}

// PrintPreflightFail prints a red cross with the given label.
func PrintPreflightFail(label string) {
	FailColor.Printf("  ✗ %s\n", label)
}
