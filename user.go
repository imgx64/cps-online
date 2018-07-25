// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"
	appengineuser "google.golang.org/appengine/user"
)

type user struct {
	Email string
	Name  string
	Roles roles

	Student *studentType // nil if not student
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

	anyRole = roles{true, true, true, true}
)

func getUser(c context.Context) (user, error) {
	u := appengineuser.Current(c)

	name := u.String()

	var stup *studentType
	var userRoles roles
	if u.Admin {
		userRoles = roles{
			Student: false,
			Admin:   true,
			HR:      true,
			Teacher: true,
		}
	} else {
		if stu, err := getStudentFromEmail(c, u.Email); err == nil {
			stup = &stu
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

		Student: stup,
	}

	return user, nil
}
