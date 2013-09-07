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
	"/users":   adminRole,
	"/reports": adminRole,
	"/backup":  adminRole,
	"/publish": adminRole,

	"/students":         hrRole,
	"/students/details": hrRole,
	"/students/save":    hrRole,
	"/students/import":  hrRole,
	"/students/export":  hrRole,

	"/payments": hrRole,

	"/employees":         hrRole,
	"/employees/details": hrRole,
	"/employees/save":    hrRole,
	"/employees/import":  hrRole,
	"/employees/export":  hrRole,

	"/classes": hrRole,

	"/marks":      teacherRole,
	"/marks/save": teacherRole,
	"/upload":     teacherRole,
	"/dailylog":   teacherRole,

	"/reportcard":   studentRole,
	"/documents":    studentRole,
	"/viewdailylog": studentRole,
}

func accessHandler(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		user, err := getUser(c)
		if err != nil {
			c.Errorf("Could not get user: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		if !canAccess(user.Roles, r.URL.Path) {
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
	{Name: "Daily Log", URL: "/dailylog"},

	{Name: "Reportcard", URL: "/reportcard"},
	//{Name: "Download Documents", URL: "/documents"},
	{Name: "Daily Log", URL: "/viewdailylog"},
}

func canAccess(userRoles roles, url string) bool {
	urlRoles := access[url]
	return (urlRoles.Student && userRoles.Student) ||
		(urlRoles.Admin && userRoles.Admin) ||
		(urlRoles.HR && userRoles.HR) ||
		(urlRoles.Teacher && userRoles.Teacher)
}
