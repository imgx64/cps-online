// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"golang.org/x/net/context"
	"google.golang.org/appengine/datastore"
	appengineuser "google.golang.org/appengine/user"
)

type user struct {
	Email string
	Name  string
	Roles roles

	Key      *datastore.Key
	Employee *employeeType // nil if not employee
	Student  *studentType  // nil if not student
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

	var userRoles roles
	var key *datastore.Key
	var empp *employeeType
	var stup *studentType
	if u.Admin {
		userRoles = roles{
			Student: false,
			Admin:   true,
			HR:      true,
			Teacher: true,
		}
		// Don't fail if admin is not employee
		if emp, err := getEmployeeFromEmail(c, u.Email); err == nil {
			key = emp.Key
			empp = &emp
		}
	} else {
		if stu, err := getStudentFromEmail(c, u.Email); err == nil {
			userRoles = roles{
				Student: true,
			}
			key = stu.Key
			stup = &stu
		} else {
			emp, err := getEmployeeFromEmail(c, u.Email)
			if err != nil {
				return user{
					Email: u.Email,
					Name:  "Unknown",
				}, err
			}
			userRoles = emp.Roles
			key = emp.Key
			empp = &emp
		}
	}

	user := user{
		Email: u.Email,
		Name:  name,
		Roles: userRoles,

		Key:      key,
		Employee: empp,
		Student:  stup,
	}

	return user, nil
}
