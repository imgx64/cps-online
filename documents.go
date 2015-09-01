// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/blobstore"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func init() {
	http.HandleFunc("/upload", accessHandler(uploadHandler))
	http.HandleFunc("/upload/file", accessHandler(uploadFileHandler))
	http.HandleFunc("/upload/link", accessHandler(uploadLinkHandler))

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

	URL string
}

func getDocuments(c context.Context, class string) ([]documentType, error) {
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

func (dt documentType) save(c context.Context) error {
	key := datastore.NewIncompleteKey(c, "document", nil)
	_, err := nds.Put(c, key, &dt)
	if err != nil {
		return err
	}

	return nil
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	uploadURL, err := blobstore.UploadURL(c, "/upload/file", nil)
	if err != nil {
		log.Errorf(c, "Could not get upload URL", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	documents, err := getDocuments(c, "all")
	if err != nil {
		log.Errorf(c, "Could not retrieve documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classes := getClasses(c, sy)

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
		log.Errorf(c, "Could not render template upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	blobs, formData, err := blobstore.ParseUpload(r)
	if err != nil {
		log.Errorf(c, "Could not parse upload: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	file := blobs["file"]
	if len(file) != 1 {
		log.Errorf(c, "No file uploaded")
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
		log.Errorf(c, "Could not save document: %s", err)
		blobstore.Delete(c, blobKey)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/upload", http.StatusFound)
}

func uploadLinkHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)
	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	title := r.Form.Get("title")
	if title == "" {
		log.Errorf(c, "No title submitted")
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	class := r.Form.Get("class")
	uploadDate := time.Now()

	fileURL := r.Form.Get("url")
	_, err := url.Parse(fileURL)
	if err != nil {
		log.Errorf(c, "Invalid URL: %s", fileURL)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	document := documentType{
		Title:      title,
		Class:      class,
		UploadDate: uploadDate,

		URL: fileURL,
	}

	if err := document.save(c); err != nil {
		log.Errorf(c, "Could not save document: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/upload", http.StatusFound)
}

func documentsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	user, err := getUser(c)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	if user.Student == nil {
		log.Errorf(c, "User is not a student: %s", user.Email)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	stu := *user.Student
	class, _, err := getStudentClass(c, stu.ID, sy)
	if err != nil {
		log.Errorf(c, "Could not get user: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	classDocuments, err := getDocuments(c, class)
	if err != nil {
		log.Errorf(c, "Could not retrieve documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	allDocuments, err := getDocuments(c, "")
	if err != nil {
		log.Errorf(c, "Could not retrieve documents: %s", err)
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
		log.Errorf(c, "Could not render template documents: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func downloadHandler(w http.ResponseWriter, r *http.Request) {
	blobstore.Send(w, appengine.BlobKey(r.FormValue("blobKey")))
}
