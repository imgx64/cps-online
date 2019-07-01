// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"fmt"
	"net/http"
	//"strconv"
	//"strings"
)

func init() {
	http.HandleFunc("/progressreports/settings", accessHandler(progressreportsSettingsHandler))
	http.HandleFunc("/progressreports/settings/save", accessHandler(progressreportsSettingsSaveHandler))
	http.HandleFunc("/progressreports/report", accessHandler(progressreportsReportHandler))
	http.HandleFunc("/progressreports/report/save", accessHandler(progressreportsReportSaveHandler))
	http.HandleFunc("/progressreports/report/print", accessHandler(progressreportsReportPrintHandler))
}

type ProgressReportSettings struct {
	SchoolYear  string
	Class       string
	ShortName   string
	Description string
	Language    string
	Rows        []ProgressReportRow
}

type ProgressReportRow struct {
	Description string
	Section     bool
	Deleted     bool
}

func getProgressReportSettings(c context.Context, sy, class, shortName string) (ProgressReportSettings, error) {
	keyStr := fmt.Sprintf("%s|%s|%s", sy, class, shortName)
	key := datastore.NewKey(c, "progress_report_settings", keyStr, 0, nil)

	var prs ProgressReportSettings
	err := nds.Get(c, key, &prs)
	if err == datastore.ErrNoSuchEntity {
		return ProgressReportSettings{}, nil
	} else if err != nil {
		return ProgressReportSettings{}, err
	}
	return prs, nil
}

func getClassProgressReportSettings(c context.Context, sy, class string) ([]ProgressReportSettings, error) {
	q := datastore.NewQuery("progress_report_settings")
	q = q.Filter("SchoolYear =", sy)
	q = q.Filter("Class =", class)
	q = q.Project("ShortName")

	var result []ProgressReportSettings
	if _, err := q.GetAll(c, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func getAllProgressReportSettings(c context.Context, sy string) (map[string][]ProgressReportSettings, error) {
	q := datastore.NewQuery("progress_report_settings")
	q = q.Filter("SchoolYear =", sy)
	q = q.Project("Class", "ShortName")

	var list []ProgressReportSettings
	if _, err := q.GetAll(c, &list); err != nil {
		return nil, err
	}

	result := make(map[string][]ProgressReportSettings)
	for _, prs := range list {
		classList := result[prs.Class]
		classList = append(classList, prs)
		result[prs.Class] = classList
	}
	return result, nil
}

func saveProgressReportSettings(c context.Context, prs ProgressReportSettings) error {
	keyStr := fmt.Sprintf("%s|%s|%s", prs.SchoolYear, prs.Class, prs.ShortName)
	key := datastore.NewKey(c, "progress_report_settings", keyStr, 0, nil)

	_, err := nds.Put(c, key, &prs)
	if err != nil {
		return err
	}
	return nil
}

func progressreportsSettingsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class := r.Form.Get("class")
	shortName := r.Form.Get("shortName")

	prs, err := getProgressReportSettings(c, sy, class, shortName)
	if err != nil {
		log.Errorf(c, "Could not get ProgressReportSettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classes := getClasses(c, sy)

	data := struct {
		Classes   []string
		Languages []string

		PRS ProgressReportSettings
	}{
		classes,
		[]string{"Arabic", "English"},

		prs,
	}

	if err := render(w, r, "progressreportsettings", data); err != nil {
		log.Errorf(c, "Could not render template progressreportsettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func progressreportsSettingsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	prs := ProgressReportSettings{
		SchoolYear:  sy,
		Class:       r.PostForm.Get("Class"),
		ShortName:   r.PostForm.Get("ShortName"),
		Description: r.PostForm.Get("Description"),
		Language:    r.PostForm.Get("Language"),
	}

	for i := 0; ; i++ {
		_, ok := r.PostForm[fmt.Sprintf("prr-description-%d", i)]
		if !ok {
			break
		}

		prr := ProgressReportRow{
			Description: r.PostForm.Get(fmt.Sprintf("prr-description-%d", i)),
			Section:     r.PostForm.Get(fmt.Sprintf("prr-type-%d", i)) == "Section",
			Deleted:     r.PostForm.Get(fmt.Sprintf("prr-type-%d", i)) == "Delete",
		}

		prs.Rows = append(prs.Rows, prr)
	}

	// Delete trailing "Deleted"
	for i := len(prs.Rows) - 1; i >= 0; i-- {
		if prs.Rows[i].Deleted {
			prs.Rows = prs.Rows[:i]
		} else {
			break
		}
	}

	if err := saveProgressReportSettings(c, prs); err != nil {
		log.Errorf(c, "Could not save ProgressReportSettings: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/settings", http.StatusFound)
}

func progressreportsReportHandler(w http.ResponseWriter, r *http.Request) {}

func progressreportsReportSaveHandler(w http.ResponseWriter, r *http.Request) {}

func progressreportsReportPrintHandler(w http.ResponseWriter, r *http.Request) {}
