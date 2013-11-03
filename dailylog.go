// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/datastore"

	"fmt"
	"net/http"
	"net/url"
	"time"
)

func init() {
	http.HandleFunc("/dailylog", accessHandler(dailylogHandler))
	http.HandleFunc("/dailylog/student", accessHandler(dailylogStudentHandler))
	http.HandleFunc("/dailylog/edit", accessHandler(dailylogEditHandler))
	http.HandleFunc("/dailylog/save", accessHandler(dailylogSaveHandler))

	http.HandleFunc("/viewdailylog", accessHandler(viewDailylogHandler))
	http.HandleFunc("/viewdailylog/day", accessHandler(viewDailylogDayHandler))
}

type dailylogType struct {
	StudentID string

	Date       time.Time
	Behavior   string
	Attendance string
	Details    string
}

func getDailylog(c appengine.Context, studentID, date string) (dailylogType, error) {
	key := datastore.NewKey(c, "dailylog", fmt.Sprintf("%s|%s", studentID, date), 0, nil)
	var dailylog dailylogType
	err := datastore.Get(c, key, &dailylog)
	if err != nil {
		return dailylogType{}, err
	}

	return dailylog, nil
}

func getDailylogs(c appengine.Context, StudentID string) ([]dailylogType, error) {
	q := datastore.NewQuery("dailylog").Filter("StudentID =", StudentID)
	var dailylogs []dailylogType
	_, err := q.GetAll(c, &dailylogs)
	if err != nil {
		return nil, err
	}

	return dailylogs, nil
}

func (dl dailylogType) save(c appengine.Context) error {
	keyStr := fmt.Sprintf("%s|%s", dl.StudentID, dl.Date.Format("2006-01-02"))
	key := datastore.NewKey(c, "dailylog", keyStr, 0, nil)
	_, err := datastore.Put(c, key, &dl)
	if err != nil {
		return err
	}

	return nil
}

func (dl dailylogType) delete(c appengine.Context) error {
	keyStr := fmt.Sprintf("%s|%s", dl.StudentID, dl.Date.Format("2006-01-02"))
	key := datastore.NewKey(c, "dailylog", keyStr, 0, nil)
	err := datastore.Delete(c, key)
	if err != nil {
		return err
	}

	return nil
}

func dailylogHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	err := r.ParseForm()
	if err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classSection := r.Form.Get("classsection")

	var students []studentType

	if classSection != "" {
		students, err = getStudents(c, true, classSection)
		if err != nil {
			c.Errorf("Could not retrieve students: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	data := struct {
		S []studentType

		CG []classGroup

		ClassSection string
	}{
		students,

		classGroups,

		classSection,
	}

	if err := render(w, r, "dailylog", data); err != nil {
		c.Errorf("Could not render template dailylog: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func dailylogStudentHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	id := r.Form.Get("id")
	stu, err := getStudent(c, id)
	if err != nil {
		c.Errorf("Could not retrieve student details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	dailylogs, err := getDailylogs(c, id)
	if err != nil {
		c.Errorf("Could not retrieve daily logs: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		S         studentType
		Today     time.Time
		Dailylogs []dailylogType
	}{
		stu,
		time.Now(),
		dailylogs,
	}

	if err := render(w, r, "dailylogstudent", data); err != nil {
		c.Errorf("Could not render template dailylogstudent: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func dailylogEditHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	id := r.Form.Get("id")
	date := r.Form.Get("date")
	if id == "" || date == "" {
		c.Errorf("Empty student (%s) or daily log (%s)", id, date)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	dailylog, err := getDailylog(c, id, date)
	if err == datastore.ErrNoSuchEntity {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			d = time.Now()
		}
		dailylog.StudentID = id
		dailylog.Date = d
	} else if err != nil {
		c.Errorf("Could not retrieve daily log details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Dailylog dailylogType
	}{
		dailylog,
	}

	if err := render(w, r, "dailylogedit", data); err != nil {
		c.Errorf("Could not render template dailylogedit: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func dailylogSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	f := r.PostForm
	c.Debugf("%#v", f)
	id := f.Get("ID")
	date, err := time.Parse("2006-01-02", f.Get("Date"))
	if err != nil {
		c.Errorf("Invalid date: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	behavior := f.Get("Behavior")
	attendance := f.Get("Attendance")
	details := f.Get("Details")

	dailylog := dailylogType{
		StudentID: id,

		Date:       date,
		Behavior:   behavior,
		Attendance: attendance,
		Details:    details,
	}

	if f.Get("submit") == "Delete" {
		err = dailylog.delete(c)
	} else {
		err = dailylog.save(c)
	}
	if err != nil {
		// TODO: message to user
		c.Errorf("Could not store dailylog: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	urlValues := url.Values{
		"id":   []string{id},
		"date": []string{date.Format("2006-01-02")},
	}
	redirectURL := fmt.Sprintf("/dailylog/student?%s", urlValues.Encode())
	http.Redirect(w, r, redirectURL, http.StatusFound)

}

func viewDailylogHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	user, err := getUser(c)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		c.Errorf("User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student

	dailylogs, err := getDailylogs(c, stu.ID)
	if err != nil {
		c.Errorf("Could not retrieve daily logs: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		S         studentType
		Today     time.Time
		Dailylogs []dailylogType
	}{
		stu,
		time.Now(),
		dailylogs,
	}

	if err := render(w, r, "viewdailylog", data); err != nil {
		c.Errorf("Could not render template viewdailylog: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func viewDailylogDayHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if err := r.ParseForm(); err != nil {
		c.Errorf("Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	user, err := getUser(c)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		c.Errorf("User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student
	id := stu.ID

	date := r.Form.Get("date")
	if id == "" || date == "" {
		c.Errorf("Empty student (%s) or daily log (%s)", id, date)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	dailylog, err := getDailylog(c, id, date)
	if err == datastore.ErrNoSuchEntity {
		d, err := time.Parse("2006-01-02", date)
		if err != nil {
			d = time.Now()
		}
		dailylog.StudentID = id
		dailylog.Date = d
	} else if err != nil {
		c.Errorf("Could not retrieve daily log details: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Dailylog dailylogType
	}{
		dailylog,
	}

	if err := render(w, r, "viewdailylogday", data); err != nil {
		c.Errorf("Could not render template viewdailylogday: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}
