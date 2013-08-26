// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// TODO: change these into configuration

var classes = []string{
	"KG1",
	"KG2",
	"1",
	"2",
	"3",
	"4",
	"5",
	"6",
	"7",
	"8",
	"9",
	"10",
	"11",
	"12",
	"SN",
}

var sections = map[string][]string{
	"KG1": {"A", "B", "C"},
	"KG2": {"A", "B", "C"},
	"1":   {"A", "B", "C"},
	"2":   {"A", "B", "C"},
	"3":   {"A", "B"},
	"4":   {"A", "B", "C"},
	"5":   {"A", "B", "C"},
	"6":   {"A", "B"},
	"7":   {"A", "B"},
	"8":   {"A", "B"},
	"9":   {""},
	"10":  {""},
	"11":  {},
	"12":  {},
	"SN":  {""},
}

type classGroup struct {
	Class    string
	Sections []string
}

var classGroups []classGroup

func init() {
	classGroups = make([]classGroup, 0, len(classes))
	for _, c := range classes {
		s, ok := sections[c]
		if !ok || len(s) == 0 {
			continue
		}
		group := classGroup{
			Class:    c,
			Sections: s,
		}
		classGroups = append(classGroups, group)
	}
}
