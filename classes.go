// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"

	"fmt"
	"strings"
)

func getClasses(c context.Context, sy string) []string {

	settings := getClassSettings(c, sy)

	classes := []string{}

	for _, setting := range settings {
		classes = append(classes, setting.Class)
	}

	return classes
}

func getClassSections(c context.Context, sy string) map[string][]string {

	settings := getClassSettings(c, sy)

	sections := make(map[string][]string, len(settings))

	for _, setting := range settings {
		sections[setting.Class] = sectionsUntil(setting.MaxSection)
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

func getClassGroups(c context.Context, sy string) []classGroup {
	// TODO: caching
	classes := getClasses(c, sy)
	sections := getClassSections(c, sy)

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
