// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"

	"net/http"
)

type link struct {
	Name   string
	URL    string
	Active bool
}

var access = map[string]role{
	"/users":   admin,
	"/reports": admin,
	"/backup":  admin,

	"/students":         hr,
	"/students/details": hr,
	"/students/save":    hr,
	"/payments":         hr,
	"/employees":        hr,
	"/classes":          hr,
	"/publish":          hr,

	"/grades":     teacher,
	"/upload":     teacher,
	"/attendance": teacher,
	"/behavior":   teacher,

	"/reportcard":     student,
	"/documents":      student,
	"/viewattendance": student,
	"/viewbehavior":   student,
}

func accessHandler(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		user, err := getUser(r)
		if err != nil {
			c.Errorf("Could not get user: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		if !can_access(user.Roles, r.URL.Path) {
			renderError(w, r, http.StatusForbidden)
			return
		}
		f(w, r)
	}
}

var pages = []link{
	{Name: "Users", URL: "/users"},
	{Name: "Reports", URL: "/reports"},
	{Name: "Backup/Restore", URL: "/backup"},

	{Name: "Students", URL: "/students"},
	{Name: "Payment Structure", URL: "/payments"},
	{Name: "Employees", URL: "/employees"},
	{Name: "Classes", URL: "/classes"},
	{Name: "Publish Reportcards", URL: "/publish"},

	{Name: "Enter Grades", URL: "/grades"},
	{Name: "Upload documents", URL: "/upload"},
	{Name: "Attendance", URL: "/attendance"},
	{Name: "Behavior", URL: "/behavior"},

	{Name: "Reportcard", URL: "/reportcard"},
	{Name: "Download Documents", URL: "/documents"},
	{Name: "Attendance", URL: "/viewattendance"},
	{Name: "Behavior", URL: "/viewbehavior"},
}

func can_access(role role, url string) bool {
	return (access[url] & role) != 0
}
