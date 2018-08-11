// Copyright 2018 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"errors"
	"fmt"
	"net/http"
	"time"
)

func init() {
	http.HandleFunc("/attendance", accessHandler(attendanceHandler))
	http.HandleFunc("/attendance/save", accessHandler(attendanceSaveHandler))
	http.HandleFunc("/attendance/import", accessHandler(attendanceImportHandler))
	http.HandleFunc("/attendance/export", accessHandler(attendanceExportHandler))
	http.HandleFunc("/attendance/report", accessHandler(attendanceReportHandler))
}

// Attendance will be stored in the datastore
type Attendance struct {
	Date     time.Time // year, month, day
	UserKey  *datastore.Key
	UserName string `datastore:"-"`

	From time.Time // hour, minute
	To   time.Time // hour, minute
}

func getGroupAttendances(c context.Context, date time.Time, group string) ([]Attendance, error) {
	var atts []Attendance

	if group == "employee" {
		employees, err := getEmployees(c, true, "all")
		if err != nil {
			return nil, err
		}
		for _, emp := range employees {
			var att Attendance
			att.UserKey = datastore.NewKey(c, "employee", "", emp.ID, nil)
			att.UserName = emp.Name
			atts = append(atts, att)
		}
	} else {
		return nil, errors.New(fmt.Sprintf("Unknown group: %s", group))
	}

	for i, att := range atts {
		var err error
		atts[i], err = getAttendance(c, date, att.UserKey, att.UserName)
		if err != nil {
			return nil, err
		}
	}

	return atts, nil
}

func getAttendance(c context.Context, date time.Time, userKey *datastore.Key, userName string) (Attendance, error) {
	keyStr := fmt.Sprintf("%s|%s", formatDate(date), userKey.Encode())
	key := datastore.NewKey(c, "attendance", keyStr, 0, nil)
	var attendance Attendance
	if err := nds.Get(c, key, &attendance); err != nil {
		if err == datastore.ErrNoSuchEntity {
			attendance.Date = date
			attendance.UserKey = userKey
		} else {
			return attendance, err
		}
	}
	attendance.UserName = userName
	return attendance, nil
}

func storeAttendance(c context.Context, attendance Attendance) error {
	keyStr := fmt.Sprintf("%s|%s", formatDate(attendance.Date), attendance.UserKey.Encode())
	key := datastore.NewKey(c, "attendance", keyStr, 0, nil)

	_, err := nds.Put(c, key, &attendance)
	return err
}

func attendanceHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	group := r.Form.Get("Group")
	if group == "" {
		group = "employee"
	}

	date, err := parseDate(r.Form.Get("Date"))
	if err != nil {
		log.Errorf(c, "Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if date.IsZero() {
		date = time.Now()
	}

	attendances, err := getGroupAttendances(c, date, group)
	if err != nil {
		log.Errorf(c, "Unable to get group attendance: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Group string
		Date  time.Time

		Attendances []Attendance
	}{
		group,
		date,

		attendances,
	}

	if err := render(w, r, "attendance", data); err != nil {
		log.Errorf(c, "Could not render template attendance: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func attendanceSaveHandler(w http.ResponseWriter, r *http.Request) {
}

func attendanceImportHandler(w http.ResponseWriter, r *http.Request) {
}

func attendanceExportHandler(w http.ResponseWriter, r *http.Request) {
}

func attendanceReportHandler(w http.ResponseWriter, r *http.Request) {
}
