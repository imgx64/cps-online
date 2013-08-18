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
	Roles role
	Links []link
}

type role uint32

var (
	student = role(0x1)

	admin   = role(0x2)
	hr      = role(0x4)
	teacher = role(0x8)

	superadmin = admin | hr | teacher
)

func getUser(r *http.Request) (user, error) {
	c := appengine.NewContext(r)
	u := appengineuser.Current(c)

	name := u.String()

	roles := role(0) //TODO
	if u.Admin {
		roles = superadmin
	}

	var links []link
	for _, page := range pages {
		if can_access(roles, page.URL) {
			if r.URL.Path == page.URL {
				page.Active = true
			}
			links = append(links, page)
		}
	}

	user := user{
		Email: u.Email,
		Name: name,
		Roles: roles,
		Links: links,
	}

	return user, nil
}
