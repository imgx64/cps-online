// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"appengine"
	"appengine/blobstore"
	"appengine/datastore"

	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/upload", accessHandler(uploadHandler))
	http.HandleFunc("/upload/do", accessHandler(uploadDoHandler))

	http.HandleFunc("/documents", accessHandler(documentsHandler))

	// no accessHander
	// has trailing slash so that files are downloaded by name
	http.HandleFunc("/download/", downloadHandler)
}

type documentType struct {
	Title      string
	Class      string
	UploadDate time.Time

	Filename string
	BlobKey  appengine.BlobKey
}

func getDocuments(c appengine.Context, class string) ([]documentType, error) {
	q := datastore.NewQuery("document")
	if class != "all" {
		q = q.Filter("Class =", class)
	}
	q = q.Order("-UploadDate")
	var documents []documentType
	_, err := q.GetAll(c, &documents)
	if err != nil {
		return nil, err
	}

	return documents, nil
}

func (dt documentType) save(c appengine.Context) error {
	key := datastore.NewIncompleteKey(c, "document", nil)
	_, err := datastore.Put(c, key, &dt)
	if err != nil {
		return err
	}

	return nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	uploadURL, err := blobstore.UploadURL(c, "/upload/do", nil)
	if err != nil {
		c.Errorf("Could not get upload URL", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	documents, err := getDocuments(c, "all")
	if err != nil {
		c.Errorf("Could not retrieve documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		UploadURL *url.URL
		Classes   []string

		Documents []documentType
	}{
		uploadURL,
		classes,

		documents,
	}

	if err := render(w, r, "upload", data); err != nil {
		c.Errorf("Could not render template upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func uploadDoHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, formData, err := blobstore.ParseUpload(r)
	if err != nil {
		c.Errorf("Could not parse upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	file := blobs["file"]
	if len(file) != 1 {
		c.Errorf("No file uploaded")
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	filename := file[0].Filename

	title := formData.Get("title")
	if title == "" {
		title = strings.TrimSuffix(filename, path.Ext(filename))
	}
	class := formData.Get("class")
	uploadDate := time.Now()
	blobKey := file[0].BlobKey

	document := documentType{
		Title:      title,
		Class:      class,
		UploadDate: uploadDate,
		Filename:   filename,
		BlobKey:    blobKey,
	}

	if err := document.save(c); err != nil {
		c.Errorf("Could not save document: %s", err)
		blobstore.Delete(c, blobKey)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/upload", http.StatusFound)
}

func documentsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	user, err := getUser(c)
	if err != nil {
		c.Errorf("Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		c.Errorf("User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student
	class := stu.Class

	classDocuments, err := getDocuments(c, class)
	if err != nil {
		c.Errorf("Could not retrieve documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	allDocuments, err := getDocuments(c, "")
	if err != nil {
		c.Errorf("Could not retrieve documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	data := struct {
		Class          string
		ClassDocuments []documentType
		AllDocuments   []documentType
	}{
		class,
		classDocuments,
		allDocuments,
	}

	if err := render(w, r, "documents", data); err != nil {
		c.Errorf("Could not render template documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}
