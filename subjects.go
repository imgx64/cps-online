// Copyright 2013 Ibrahim Ghazal. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"github.com/qedus/nds"
	"golang.org/x/net/context"
	"google.golang.org/appengine"
	"google.golang.org/appengine/datastore"
	"google.golang.org/appengine/log"

	"fmt"
	"net/http"
	"strconv"
)

func init() {
	http.HandleFunc("/subjects", accessHandler(subjectsHandler))
	http.HandleFunc("/subjects/details", accessHandler(subjectsDetailsHandler))
	http.HandleFunc("/subjects/save", accessHandler(subjectsSaveHandler))
}

type subjectSetting struct {
	Value []string
}

func getSubjects(c context.Context, sy, class string) ([]string, error) {
	key := datastore.NewKey(c, "settings",
		fmt.Sprintf("classsubjects-%s-%s", sy, class), 0, nil)

	setting := subjectSetting{}
	err := nds.Get(c, key, &setting)
	if err == datastore.ErrNoSuchEntity {
		return []string{}, nil
	} else if err != nil {
		return nil, err
	}

	return setting.Value, nil
}

func saveSubjects(c context.Context, sy, class string, subjects []string) error {
	key := datastore.NewKey(c, "settings",
		fmt.Sprintf("classsubjects-%s-%s", sy, class), 0, nil)
	_, err := nds.Put(c, key, &subjectSetting{subjects})
	if err != nil {
		return err
	}
	return nil
}

func getSubject(c context.Context, sy, class, subjectname string) (Subject, error) {
	key := datastore.NewKey(c, "subjects",
		fmt.Sprintf("%s-%s-%s", sy, class, subjectname), 0, nil)

	var subject Subject
	err := nds.Get(c, key, &subject)
	if err != nil {
		return Subject{}, err
	}

	if subject.SemesterType == 0 {
		subject.SemesterType = Quarterly
	}

	for i, gc := range subject.QuarterGradingColumns {
		if gc.FinalWeight == 0 {
			if gc.Type == quizGrading {
				gc.FinalWeight = gc.Max * float64(gc.BestQuizzes)
			} else {
				gc.FinalWeight = gc.Max
			}
		}
		subject.QuarterGradingColumns[i] = gc
	}
	for i, gc := range subject.SemesterGradingColumns {
		if gc.FinalWeight == 0 {
			if gc.Type == quizGrading {
				gc.FinalWeight = gc.Max * float64(gc.BestQuizzes)
			} else {
				gc.FinalWeight = gc.Max
			}
		}
		subject.SemesterGradingColumns[i] = gc
	}

	return subject, nil
}

func saveSubject(c context.Context, sy, class string, subject Subject) error {
	if subject.ShortName == "" {
		return fmt.Errorf("Subject does not have a short name: %s", subject.ShortName)
	}

	if subject.Description == "" {
		subject.Description = subject.ShortName
	}

	found := false
	for _, s := range getAllSubjects(c, sy) {
		if subject.ShortName == s {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Subject does not exist: %s", subject.ShortName)
	}

	qTotal := 0.0
	for _, gc := range subject.QuarterGradingColumns {
		qTotal += gc.FinalWeight
	}
	if qTotal != 0.0 && qTotal != 100.0 {
		return fmt.Errorf("Total marks for quarter must be 100. Got %f", qTotal)
	}

	sTotal := 0.0
	for _, gc := range subject.SemesterGradingColumns {
		sTotal += gc.FinalWeight
	}

	if sTotal != 0.0 && sTotal != 100.0 {
		return fmt.Errorf("Total marks for semester must be 100. Got %f", sTotal)
	}

	if qTotal == 0.0 && sTotal == 0.0 {
		return fmt.Errorf("Please add columns")
	}

	key := datastore.NewKey(c, "subjects",
		fmt.Sprintf("%s-%s-%s", sy, class, subject.ShortName), 0, nil)

	_, err := nds.Put(c, key, &subject)
	if err != nil {
		return err
	}

	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		return err
	}
	found = false
	for _, s := range subjects {
		if subject.ShortName == s {
			found = true
			break
		}
	}
	if !found {
		subjects = append(subjects, subject.ShortName)
		err = saveSubjects(c, sy, class, subjects)
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteSubject(c context.Context, sy, class, subjectname string) error {
	key := datastore.NewKey(c, "subjects",
		fmt.Sprintf("%s-%s-%s", sy, class, subjectname), 0, nil)

	err := nds.Delete(c, key)
	if err != nil {
		return err
	}

	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		return err
	}
	var newSubjects []string
	for _, s := range subjects {
		if subjectname != s {
			newSubjects = append(newSubjects, s)
		}
	}
	err = saveSubjects(c, sy, class, newSubjects)
	if err != nil {
		return err
	}

	return nil
}

type classSubjects struct {
	Class    string
	Subjects []string
}

func subjectsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	var cs []classSubjects
	for _, class := range getClasses(c, sy) {
		subjects, err := getSubjects(c, sy, class)
		if err != nil {
			log.Errorf(c, "could not get class subjects %s %s: %s", sy, class, err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
		cs = append(cs, classSubjects{class, subjects})
	}

	data := struct {
		ClassSubjects []classSubjects
	}{
		cs,
	}

	if err := render(w, r, "subjects", data); err != nil {
		log.Errorf(c, "Could not render template subjects: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

type gradingColumnChoice struct {
	Value gradingColumnType
	Name  string
}

func subjectsDetailsHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class := r.Form.Get("class")
	subjectname := r.Form.Get("subject")

	subject := Subject{}
	if subjectname != "" {
		var err error
		subject, err = getSubject(c, sy, class, subjectname)
		if err != nil {
			log.Errorf(c, "Could not get subject %s, %s, %s: %s", sy, class, subjectname, err)
			renderError(w, r, http.StatusInternalServerError)
			return
		}
	}

	tempGCs := subject.QuarterGradingColumns
	if len(tempGCs) < 10 {
		tempGCs = append(tempGCs, make([]gradingColumn, 10-len(tempGCs))...)
	}
	subject.QuarterGradingColumns = tempGCs

	tempGCs = subject.SemesterGradingColumns
	if len(tempGCs) < 10 {
		tempGCs = append(tempGCs, make([]gradingColumn, 10-len(tempGCs))...)
	}
	subject.SemesterGradingColumns = tempGCs

	subjectsMap := make(map[string]bool)
	subjects, err := getSubjects(c, sy, class)
	if err != nil {
		log.Errorf(c, "Could not get subjects %s, %s: %s", sy, class, err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
	for _, s := range subjects {
		subjectsMap[s] = true
	}

	var availableSubjects []string
	for _, s := range getAllSubjects(c, sy) {
		if !subjectsMap[s] {
			availableSubjects = append(availableSubjects, s)
		}
	}

	gradingColumnChoices := []gradingColumnChoice{
		{noGrading, gradingColumnTypeStrings[noGrading]},
		{directGrading, gradingColumnTypeStrings[directGrading]},
		{quizGrading, gradingColumnTypeStrings[quizGrading]},
	}

	data := struct {
		AvailableSubjects    []string
		GradingColumnChoices []gradingColumnChoice
		SemesterTypes        []semesterType

		Class   string
		Subject Subject
	}{
		availableSubjects,
		gradingColumnChoices,
		semesterTypes,

		class,
		subject,
	}

	if err := render(w, r, "subjectsdetails", data); err != nil {
		log.Errorf(c, "Could not render template subjectsdetails: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}
}

func subjectsSaveHandler(w http.ResponseWriter, r *http.Request) {
	c := appengine.NewContext(r)

	sy := getSchoolYear(c)

	if err := r.ParseForm(); err != nil {
		log.Errorf(c, "Could not parse form: %s", err)
		renderError(w, r, http.StatusInternalServerError)
		return
	}

	class := r.PostForm.Get("Class")

	if r.PostForm.Get("submit") == "Delete" {

		subjectname := r.PostForm.Get("ShortName")

		err := deleteSubject(c, sy, class, subjectname)
		if err != nil {
			log.Errorf(c, "could not delete subject %s %s %s: %s", sy, class, subjectname, err)
			renderErrorMsg(w, r, http.StatusBadRequest, err.Error())
			return
		}

		// TODO: message of success
		http.Redirect(w, r, "/subjects", http.StatusFound)
		return
	}

	// TODO: check class exists

	var subject Subject

	subject.ShortName = r.PostForm.Get("ShortName")
	subject.Description = r.PostForm.Get("Description")
	subject.CalculateInAverage = r.PostForm.Get("CalculateInAverage") == "on"
	s1credits, err := strconv.ParseFloat(r.PostForm.Get("S1Credits"), 64)
	if err != nil {
		renderErrorMsg(w, r, http.StatusBadRequest,
			fmt.Sprintf("Invalid Semester 1 credits: %s", r.PostForm.Get("S1Credits")))
		return
	}
	subject.S1Credits = s1credits

	s2credits, err := strconv.ParseFloat(r.PostForm.Get("S2Credits"), 64)
	if err != nil {
		renderErrorMsg(w, r, http.StatusBadRequest,
			fmt.Sprintf("Invalid Semester 2 credits: %s", r.PostForm.Get("S2Credits")))
		return
	}
	subject.S2Credits = s2credits

	semType, err := strconv.Atoi(r.PostForm.Get("SemesterType"))
	if err != nil {
		renderErrorMsg(w, r, http.StatusBadRequest,
			fmt.Sprintf("Invalid Semester Type: %s", r.PostForm.Get("SemesterType")))
		return
	}
	subject.SemesterType = semesterType(semType)

	for i := 0; ; i++ {
		typeStr := r.PostForm.Get(fmt.Sprintf("qgc-type-%d", i))
		nameStr := r.PostForm.Get(fmt.Sprintf("qgc-name-%d", i))
		maxStr := r.PostForm.Get(fmt.Sprintf("qgc-max-%d", i))
		weightStr := r.PostForm.Get(fmt.Sprintf("qgc-weight-%d", i))
		numQuizzesStr := r.PostForm.Get(fmt.Sprintf("qgc-num-quizzes-%d", i))
		bestQuizzesStr := r.PostForm.Get(fmt.Sprintf("qgc-best-quizzes-%d", i))

		typeInt, err := strconv.Atoi(typeStr)
		if err != nil {
			break
		}
		typ := gradingColumnType(typeInt)

		name := nameStr
		if name == "" {
			continue
		}

		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}
		if max == 0 {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}

		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Final Weight for %s: %s", name, weightStr))
			return
		}
		if weight == 0 {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Final Weight for %s: %s", name, weightStr))
			return
		}

		var numQuizzes, bestQuizzes int
		if typ == directGrading {
			// no special handling
		} else if typ == quizGrading {
			numQuizzes, err = strconv.Atoi(numQuizzesStr)
			if err != nil || numQuizzes < 1 {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Invalid Number of Quizzes for %s: %s", name, numQuizzesStr))
				return
			}

			bestQuizzes, err = strconv.Atoi(bestQuizzesStr)
			if err != nil || bestQuizzes < 1 {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Invalid Best Quizzes for %s: %s", name, bestQuizzesStr))
				return
			}

			if bestQuizzes > numQuizzes {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Best Quizzes for %s are greater than Number of Quizzes: %s", name, bestQuizzesStr))
				return
			}
		} else {
			// Unused
			continue
		}

		gc := gradingColumn{
			typ,
			name,
			max,
			weight,
			numQuizzes,
			bestQuizzes,
		}

		subject.QuarterGradingColumns = append(subject.QuarterGradingColumns, gc)
	}

	for i := 0; ; i++ {
		typeStr := r.PostForm.Get(fmt.Sprintf("sgc-type-%d", i))
		nameStr := r.PostForm.Get(fmt.Sprintf("sgc-name-%d", i))
		maxStr := r.PostForm.Get(fmt.Sprintf("sgc-max-%d", i))
		weightStr := r.PostForm.Get(fmt.Sprintf("sgc-weight-%d", i))
		numQuizzesStr := r.PostForm.Get(fmt.Sprintf("sgc-num-quizzes-%d", i))
		bestQuizzesStr := r.PostForm.Get(fmt.Sprintf("sgc-bestquizzes-%d", i))

		typeInt, err := strconv.Atoi(typeStr)
		if err != nil {
			break
		}
		typ := gradingColumnType(typeInt)

		name := nameStr
		if name == "" {
			continue
		}

		max, err := strconv.ParseFloat(maxStr, 64)
		if err != nil {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}
		if max == 0 {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Encoded Max for %s: %s", name, maxStr))
			return
		}

		weight, err := strconv.ParseFloat(weightStr, 64)
		if err != nil {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Final Weight for %s: %s", name, weightStr))
			return
		}
		if weight == 0 {
			renderErrorMsg(w, r, http.StatusBadRequest,
				fmt.Sprintf("Invalid Final Weight for %s: %s", name, weightStr))
			return
		}

		var numQuizzes, bestQuizzes int
		if typ == quizGrading {
			numQuizzes, err := strconv.Atoi(numQuizzesStr)
			if err != nil || numQuizzes < 1 {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Invalid Number of Quizzes for %s: %s", name, numQuizzesStr))
				return
			}

			bestQuizzes, err := strconv.Atoi(bestQuizzesStr)
			if err != nil || bestQuizzes < 1 {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Invalid Best Quizzes for %s: %s", name, bestQuizzesStr))
				return
			}

			if bestQuizzes > numQuizzes {
				renderErrorMsg(w, r, http.StatusBadRequest,
					fmt.Sprintf("Best Quizzes for %s are greater than Number of Quizzes: %s", name, bestQuizzesStr))
				return
			}
		}

		gc := gradingColumn{
			typ,
			name,
			max,
			weight,
			numQuizzes,
			bestQuizzes,
		}

		subject.SemesterGradingColumns = append(subject.SemesterGradingColumns, gc)
	}

	err = saveSubject(c, sy, class, subject)
	if err != nil {
		log.Errorf(c, "could not save subject %s %s %v: %s", sy, class, subject, err)
		renderErrorMsg(w, r, http.StatusBadRequest, err.Error())
		return
	}

	// TODO: message of success
	http.Redirect(w, r, "/subjects", http.StatusFound)
}
