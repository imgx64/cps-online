// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"

	"fmt"
	"strings"
)

// TODO: change these into configuration

func getClasses(c context.Context) []string {
	// TODO: caching

	classes := []string{
		"1",
		"2",
		"3",
		"4",
		"5",
		"6",
		"7",
		"8",
		"9sci",
		"9com",
		"10sci",
		"10com",
		"11sci",
		"11com",
		"12sci",
		"12com",
		"SN",
		"PreKG",
		"KG1",
		"KG2",
	}

	return classes
}

func getClassSections(c context.Context) map[string][]string {
	// TODO: caching

	maxSections := getMaxSections(c)

	sections := make(map[string][]string, len(maxSections))

	for _, maxSection := range maxSections {
		sections[maxSection.Class] = sectionsUntil(maxSection.Section)
	}

	return sections
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

func getClassGroups(c context.Context) []classGroup {
	// TODO: caching
	classes := getClasses(c)
	sections := getClassSections(c)

	classGroups := make([]classGroup, 0, len(classes))
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
	return classGroups
}
