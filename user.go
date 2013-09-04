// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	appengineuser "appengine/user"

	"net/http"
)

type user struct {
	Email string
	Name  string
	Roles roles
	Links []link
}

type roles struct {
	Student bool

	Admin   bool
	HR      bool
	Teacher bool
}

var (
	studentRole = roles{Student: true}

	adminRole   = roles{Admin: true}
	hrRole      = roles{HR: true}
	teacherRole = roles{Teacher: true}
)

func getUser(r *http.Request) (user, error) {
	c := appengine.NewContext(r)
	u := appengineuser.Current(c)

	name := u.String()

	var userRoles roles
	if u.Admin {
		userRoles = roles{
			Student: false,
			Admin:   true,
			HR:      true,
			Teacher: true,
		}
	} else {
		if isStudentEmail(r, u.Email) {
			userRoles = roles{
				Student: true,
			}
		} else {
			emp, err := getEmployeeFromEmail(r, u.Email)
			if err != nil {
				return user{
					Email: u.Email,
					Name:  "Unknown",
				}, err
			}
			userRoles = emp.Roles
		}
	}

	var links []link
	for _, page := range pages {
		if canAccess(userRoles, page.URL) {
			if r.URL.Path == page.URL {
				page.Active = true
			}
			links = append(links, page)
		}
	}

	user := user{
		Email: u.Email,
		Name:  name,
		Roles: userRoles,
		Links: links,
	}

	return user, nil
}
