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

	Employee *employeeType // nil if not employee
	Student  *studentType  // nil if not student
}

func (user user) Key() *datastore.Key {
	if user.Employee != nil {
		return user.Employee.Key
	}
	if user.Student != nil {
		return user.Student.Key
	}
	return nil
}

func (user user) FullName() string {
	if user.Employee != nil {
		return user.Employee.Name
	}
	if user.Student != nil {
		return user.Student.Name
	}
	return ""
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
			empp = &emp
		}
	} else {
		if stu, err := getStudentFromEmail(c, u.Email); err == nil {
			userRoles = roles{
				Student: true,
			}
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
			empp = &emp
		}
	}

	user := user{
		Email: u.Email,
		Name:  name,
		Roles: userRoles,

		Employee: empp,
		Student:  stup,
	}

	return user, nil
}
