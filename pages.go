// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"google.golang.org/appengine"
	"google.golang.org/appengine/log"

	"net/http"
)

type link struct {
	Name   string
	URL    string
	Active bool
}

var access = map[string]roles{

	"/settings":                adminRole,
	"/settings/saveschoolyear": adminRole,
	"/settings/savesections":   adminRole,
	"/settings/addclass":       adminRole,
	"/settings/addschoolyear":  adminRole,
	"/settings/addsubject":     adminRole,
	"/settings/deletesubject":  adminRole,
	"/settings/access":         adminRole,

	"/assign":      adminRole,
	"/assign/save": adminRole,

	"/subjects":         adminRole,
	"/subjects/details": adminRole,
	"/subjects/save":    adminRole,
	"/subjects/delete":  adminRole,

	"/students":         hrRole,
	"/students/details": hrRole,
	"/students/save":    hrRole,
	"/students/import":  hrRole,
	"/students/export":  hrRole,

	"/employees":         hrRole,
	"/employees/details": hrRole,
	"/employees/save":    hrRole,
	"/employees/import":  hrRole,
	"/employees/export":  hrRole,

	"/completion":        hrRole,
	"/printallmarks":     hrRole,
	"/printstudentmarks": hrRole,
	"/reportcards":       hrRole,
	"/reportcards/print": hrRole,
	"/gpareportcard":     hrRole,

	"/marks":        teacherRole,
	"/marks/save":   teacherRole,
	"/marks/import": teacherRole,
	"/marks/export": teacherRole,

	"/homework":        teacherRole,
	"/homework/save":   teacherRole,
	"/homework/delete": teacherRole,

	"/upload":           teacherRole,
	"/upload/file":      teacherRole,
	"/upload/link":      teacherRole,
	"/dailylog":         teacherRole,
	"/dailylog/student": teacherRole,
	"/dailylog/edit":    teacherRole,
	"/dailylog/save":    teacherRole,

	"/reportcard":       studentRole,
	"/documents":        studentRole,
	"/viewdailylog":     studentRole,
	"/viewdailylog/day": studentRole,
	"/homeworks":        studentRole,
}

func accessHandler(f func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		c := appengine.NewContext(r)
		user, err := getUser(c)
		if err != nil {
			log.Errorf(c, "Could not get user: %s", err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}

		if !user.Roles.Admin {
			staffAccess := getStaffAccess(c)
			if !staffAccess {
				renderErrorMsg(w, r, http.StatusForbidden, "The system is currently in Maintenance. Please try again later.")
				return
			}
		}

		if !canAccess(user.Roles, r.URL.Path) {
			renderError(w, r, http.StatusForbidden)
			return
		}
		f(w, r)
	}
}

var pages = []link{

	{Name: "Students", URL: "/students"},
	{Name: "Employees", URL: "/employees"},
	{Name: "Assign Teachers", URL: "/assign"},
	{Name: "Check Completion", URL: "/completion"},
	{Name: "Print All Marks", URL: "/printallmarks"},

	{Name: "Enter Marks", URL: "/marks"},
	{Name: "Homework", URL: "/homework"},
	{Name: "Upload documents", URL: "/upload"},
	{Name: "Daily Log", URL: "/dailylog"},
	{Name: "Print Reportcards", URL: "/reportcards"},
	{Name: "Settings", URL: "/settings"},
	{Name: "Subjects", URL: "/subjects"},

	{Name: "Reportcard", URL: "/reportcard"},
	{Name: "Download Documents", URL: "/documents"},
	{Name: "Daily Log", URL: "/viewdailylog"},
	{Name: "Homework", URL: "/homeworks"},
}

func canAccess(userRoles roles, url string) bool {
	urlRoles := access[url]
	return (urlRoles.Student && userRoles.Student) ||
		(urlRoles.Admin && userRoles.Admin) ||
		(urlRoles.HR && userRoles.HR) ||
		(urlRoles.Teacher && userRoles.Teacher)
}
