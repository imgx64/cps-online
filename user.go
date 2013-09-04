// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	appengineuser "appengine/user"
)

type user struct {
	Email string
	Name  string
	Roles roles
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

func getUser(c appengine.Context) (user, error) {
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
		if isStudentEmail(c, u.Email) {
			userRoles = roles{
				Student: true,
			}
		} else {
			emp, err := getEmployeeFromEmail(c, u.Email)
			if err != nil {
				return user{
					Email: u.Email,
					Name:  "Unknown",
				}, err
			}
			userRoles = emp.Roles
		}
	}

	user := user{
		Email: u.Email,
		Name:  name,
		Roles: userRoles,
	}

	return user, nil
}
