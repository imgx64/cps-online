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

var access = map[string]roles{
	"/users":   admin_role,
	"/reports": admin_role,
	"/backup":  admin_role,
	"/publish": admin_role,

	"/students":         hr_role,
	"/students/details": hr_role,
	"/students/save":    hr_role,
	"/students/import":  hr_role,
	"/students/export":  hr_role,

	"/payments": hr_role,

	"/employees":         hr_role,
	"/employees/details": hr_role,
	"/employees/save":    hr_role,
	"/employees/import":  hr_role,
	"/employees/export":  hr_role,

	"/classes": hr_role,

	"/marks":      teacher_role,
	"/marks/save": teacher_role,
	"/upload":     teacher_role,
	"/attendance": teacher_role,
	"/behavior":   teacher_role,

	"/reportcard":     student_role,
	"/documents":      student_role,
	"/viewattendance": student_role,
	"/viewbehavior":   student_role,
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
	//{Name: "Users", URL: "/users"},
	//{Name: "Reports", URL: "/reports"},
	//{Name: "Backup/Restore", URL: "/backup"},

	{Name: "Students", URL: "/students"},
	//{Name: "Payment Structure", URL: "/payments"},
	{Name: "Employees", URL: "/employees"},
	//{Name: "Classes", URL: "/classes"},
	//{Name: "Publish Reportcards", URL: "/publish"},

	{Name: "Enter Marks", URL: "/marks"},
	//{Name: "Upload documents", URL: "/upload"},
	//{Name: "Attendance", URL: "/attendance"},
	//{Name: "Behavior", URL: "/behavior"},

	//{Name: "Reportcard", URL: "/reportcard"},
	//{Name: "Download Documents", URL: "/documents"},
	//{Name: "Attendance", URL: "/viewattendance"},
	//{Name: "Behavior", URL: "/viewbehavior"},
}

func can_access(userRoles roles, url string) bool {
	urlRoles := access[url]
	return (urlRoles.Student && userRoles.Student) ||
		(urlRoles.Admin && userRoles.Admin) ||
		(urlRoles.HR && userRoles.HR) ||
		(urlRoles.Teacher && userRoles.Teacher)
}
