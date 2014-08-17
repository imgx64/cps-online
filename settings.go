// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"net/http"
	"strings"
)

func init() {
	http.HandleFunc("/settings", accessHandler(settingsHandler))
	http.HandleFunc("/settings/save", accessHandler(settingsSaveHandler))
}

type schoolYearSetting struct {
	Value string
}

func getSchoolYear(c appengine.Context) string {

	key := datastore.NewKey(c, "settings", "school_year", 0, nil)

	setting := schoolYearSetting{}
	err := datastore.Get(c, key, &setting)
	var sy string
	if err == nil {
		sy = setting.Value
	} else {
		c.Warningf("Could not get school year: %s\nUsing defaults instead", err)
		sy = "2014-2015"
	}

	return sy
}

func saveSchoolYear(c appengine.Context, sy string) error {

	key := datastore.NewKey(c, "settings", "school_year", 0, nil)

	_, err := datastore.Put(c, key, &schoolYearSetting{sy})
	if err != nil {
		return err
	}
	return nil
}

type maxSection struct {
	Class   string
	Section string
}

type maxSectionSetting struct {
	Value []maxSection
}

func getMaxSections(c appengine.Context) []maxSection {

	key := datastore.NewKey(c, "settings", "sections", 0, nil)

	setting := maxSectionSetting{}
	err := datastore.Get(c, key, &setting)
	var storedMaxSections []maxSection
	if err == nil {
		storedMaxSections = setting.Value
	} else {
		c.Warningf("Could not get max sections: %s\nUsing defaults instead", err)
		storedMaxSections = []maxSection{}
	}
	// Now we have a valid storedMaxSections or an empty array

	maxSectionsMap := make(map[string]string)
	for _, maxSection := range storedMaxSections {
		maxSectionsMap[maxSection.Class] = maxSection.Section
	}

	classes := getClasses(c)
	maxSections := make([]maxSection, 0, len(classes))
	for _, class := range classes {
		section, ok := maxSectionsMap[class]
		if !ok {
			section = "D"
		}
		maxSections = append(maxSections, maxSection{class, section})
	}

	return maxSections
}

func saveMaxSections(c appengine.Context, maxSections []maxSection) error {

	key := datastore.NewKey(c, "settings", "sections", 0, nil)
	_, err := datastore.Put(c, key, &maxSectionSetting{maxSections})
	if err != nil {
		return err
	}
	return nil
}

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sectionChoices := sectionsUntil("Z")

	sy := getSchoolYear(c)

	maxSections := getMaxSections(c)

	data := struct {
		SectionChoices []string

		SY string

		MaxSections []maxSection
	}{
		sectionChoices,

		sy,

		maxSections,
	}

	if err := render(w, r, "settings", data); err != nil {
		c.Errorf("Could not render template upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func settingsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	sy := r.Form.Get("sy")
	if sy != "" {
		if err := saveSchoolYear(c, sy); err != nil {
			c.Errorf("Could not save school year: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	classes := getClasses(c)
	maxSections := make([]maxSection, 0, len(classes))
	for _, class := range classes {
		section := r.Form.Get("sections-" + class)
		if !validSection(section) {
			section = "D"
		}
		maxSections = append(maxSections, maxSection{class, section})
	}

	if err := saveMaxSections(c, maxSections); err != nil {
		c.Errorf("Could not save max sections: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/settings", http.StatusFound)
}

func validSection(section string) bool {
	if len(section) != 1 {
		return false
	}

	section = strings.ToUpper(section)
	r := rune(section[0])
	if r < 'A' || r > 'Z' {
		return false
	}

	return true
}

func sectionsUntil(end string) []string {
	if !validSection(end) {
		return nil
	}

	end = strings.ToUpper(end)
	e := rune(end[0])

	sections := make([]string, 0, e-'A'+1)
	for section := 'A'; section <= e; section++ {
		sections = append(sections, string(section))
	}

	return sections
}
