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
	student_role = roles{Student: true}

	admin_role   = roles{Admin: true}
	hr_role      = roles{HR: true}
	teacher_role = roles{Teacher: true}
)

func getUser(r *http.Request) (user, error) {
	c := appengine.NewContext(r)
	u := appengineuser.Current(c)

	name := u.String()

	userRoles := roles{} //TODO
	if u.Admin {
		userRoles = roles{
			Student: false,
			Admin:   true,
			HR:      true,
			Teacher: true,
		}
	}

	var links []link
	for _, page := range pages {
		if can_access(userRoles, page.URL) {
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
