// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	appengineuser "appengine/user"

	"net/http"
)

func init() {
	http.HandleFunc("/", root)
	http.HandleFunc("/logout", logout)
}

func root(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	if r.URL.Path != "/" {
		renderError(w, r, http.StatusNotFound)
		return
	}

	if err := render(w, r, "root", nil); err != nil {
		c.Errorf("Could not render template root: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func logout(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	logoutURL, err := appengineuser.LogoutURL(c, "/")
	if err != nil {
		c.Errorf("Could not get logout URL: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, logoutURL, http.StatusFound)
}
