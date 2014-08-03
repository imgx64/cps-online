// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strings"
)

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
	"KG1": {"A", "B", "C", "D"},
	"KG2": {"A", "B", "C", "D"},
	"1":   {"A", "B", "C", "D"},
	"2":   {"A", "B", "C", "D"},
	"3":   {"A", "B", "C", "D"},
	"4":   {"A", "B", "C", "D"},
	"5":   {"A", "B", "C", "D"},
	"6":   {"A", "B", "C", "D"},
	"7":   {"A", "B", "C", "D"},
	"8":   {"A", "B", "C", "D"},
	"9":   {"A", "B", "C", "D"},
	"10":  {"A", "B", "C", "D"},
	"11":  {"A", "B", "C", "D"},
	"12":  {"A", "B", "C", "D"},
	"SN":  {"A", "B", "C", "D"},
}

func parseClassSection(classSection string) (class, section string, err error) {
	cs := strings.Split(classSection, "|")
	if len(cs) != 2 {
		return "", "", fmt.Errorf("Unable to parse class and section: %s", classSection)
	}
	class = cs[0]
	section = cs[1]
	return class, section, nil
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
