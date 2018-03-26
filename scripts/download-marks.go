// +build !appengine

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func parseClassSection(classSection string) (class, section string, err error) {
	cs := strings.Split(classSection, "|")
	if len(cs) != 2 {
		return "", "", fmt.Errorf("Unable to parse class and section: %s", classSection)
	}
	class = cs[0]
	section = cs[1]
	return class, section, nil
}

// Copied from gradingsystems.go

type termType int

const (
	Quarter termType = iota + 1
	Semester
	EndOfYear
	EndOfYearGpa
	Midterm
	WeekS1
	WeekS2
)

var termStrings = map[termType]string{
	Quarter:      "Quarter",
	Semester:     "Semester",
	EndOfYear:    "End of Year",
	EndOfYearGpa: "End of Year (GPA)",
	Midterm:      "Midterm",
	WeekS1:       "S1 Week",
	WeekS2:       "S2 Week",
}

type Term struct {
	Typ termType
	N   int
}

func parseTerm(s string) (Term, error) {
	cs := strings.Split(s, "|")
	if len(cs) != 2 {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	typeNumber, err := strconv.Atoi(cs[0])
	if err != nil {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}
	typ := termType(typeNumber)
	_, ok := termStrings[typ]
	if !ok {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	n, err := strconv.Atoi(cs[1])
	if err != nil {
		return Term{}, fmt.Errorf("Invalid term: %s", s)
	}

	return Term{typ, n}, nil
}

// Value is used in forms
func (t Term) Value() string {
	return fmt.Sprintf("%d|%d", t.Typ, t.N)
}

// Used in reportcards template
func (t Term) ShowBehaviorReportCard() bool {
	return t.Typ == Quarter || t.Typ == Midterm
}

func (t Term) String() string {
	s, ok := termStrings[t.Typ]
	if !ok {
		panic(fmt.Sprintf("Invalid term type: %d", t.Typ))
	}
	if t.N == 0 {
		return s
	}
	return fmt.Sprintf("%s %d", s, t.N)
}

// End - Copied from gradingsystems.go

type Terms []Term

func (t Terms) Len() int {
	return len(t)
}

func (t Terms) Less(i, j int) bool {
	t1 := t[i]
	t2 := t[j]

	if t1.Typ == t2.Typ {
		return t1.N < t2.N
	}

	if t1.Typ == WeekS1 {
		return true
	}
	if t2.Typ == WeekS1 {
		return false
	}

	if t1.Typ == Quarter && t1.N == 1 {
		return true
	}
	if t2.Typ == Quarter && t2.N == 1 {
		return false
	}

	if t1.Typ == Midterm && t1.N == 1 {
		return true
	}
	if t2.Typ == Midterm && t2.N == 1 {
		return false
	}

	if t1.Typ == Quarter && t1.N == 2 {
		return true
	}
	if t2.Typ == Quarter && t2.N == 2 {
		return false
	}

	if t1.Typ == Semester && t1.N == 1 {
		return true
	}
	if t2.Typ == Semester && t2.N == 1 {
		return false
	}

	if t1.Typ == WeekS2 {
		return true
	}
	if t2.Typ == WeekS2 {
		return false
	}

	if t1.Typ == Quarter && t1.N == 3 {
		return true
	}
	if t2.Typ == Quarter && t2.N == 3 {
		return false
	}

	if t1.Typ == Midterm && t1.N == 2 {
		return true
	}
	if t2.Typ == Midterm && t2.N == 2 {
		return false
	}

	if t1.Typ == Quarter && t1.N == 4 {
		return true
	}
	if t2.Typ == Quarter && t2.N == 4 {
		return false
	}

	if t1.Typ == Semester && t1.N == 2 {
		return true
	}
	if t2.Typ == Semester && t2.N == 2 {
		return false
	}

	if t1.Typ == Quarter {
		return true
	}
	if t2.Typ == Quarter {
		return false
	}

	if t1.Typ == Midterm {
		return true
	}
	if t2.Typ == Midterm {
		return false
	}

	if t1.Typ == Semester {
		return true
	}
	if t2.Typ == Semester {
		return false
	}

	if t1.Typ == EndOfYear {
		return true
	}
	if t2.Typ == EndOfYear {
		return false
	}

	if t1.Typ == EndOfYearGpa {
		return true
	}
	if t2.Typ == EndOfYearGpa {
		return false
	}

	panic(fmt.Sprintf("Invalid terms: %s %s", t1, t2))

}

func (t Terms) Swap(i, j int) {
	t[i], t[j] = t[j], t[i]
}

func main() {
	var err error

	defer func() {
		err := recover()
		if err != nil {
			fmt.Println("Error:", err)
		}
		fmt.Println("Press Enter to close")
		fmt.Scanln()
	}()

	fmt.Print("Enter cookie value: ")
	var rawCookies string
	if _, err = fmt.Scanln(&rawCookies); err != nil {
		panic(err)
	}

	rawCookies = strings.Trim(rawCookies, " \t\n")
	rawCookies = strings.TrimPrefix(rawCookies, "cookie:")

	rawRequest := fmt.Sprintf("GET / HTTP/1.0\r\nCookie: %s\r\n\r\n", rawCookies)
	req, err := http.ReadRequest(bufio.NewReader(strings.NewReader(rawRequest)))
	if err != nil {
		panic(err)
	}
	cookies := req.Cookies()

	http.DefaultClient.Jar, err = cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	baseUrl, err := url.Parse("https://creativity-private-school-2015.appspot.com/")
	if err != nil {
		panic(err)
	}
	http.DefaultClient.Jar.SetCookies(baseUrl, cookies)

	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != via[len(via)-1].URL.Host {
			return fmt.Errorf("Invalid cookie. Redirecting to new host: %s", req.URL.Host)
		}
		return nil
	}

	subjectsMapUrl, err := url.Parse("https://creativity-private-school-2015.appspot.com/subjectsmap")
	if err != nil {
		panic(err)
	}

	fmt.Println("Getting terms, classes, and subjects...")
	resp, err := http.Get(subjectsMapUrl.String())
	if err != nil {
		panic(err)
	}

	if resp.StatusCode != 200 {
		panic(resp)
	}

	// term -> class -> subject
	subjectsMap := make(map[string]map[string][]string)
	if err := json.NewDecoder(resp.Body).Decode(&subjectsMap); err != nil {
		panic(err)
	}

	resp.Body.Close()

	var terms Terms
	for termValue, _ := range subjectsMap {
		term, err := parseTerm(termValue)
		if err != nil {
			panic(fmt.Sprintf("Invalid term: %s", termValue))
		}
		terms = append(terms, term)
	}

	sort.Sort(terms)

	fmt.Println("Select term to download")
	i := 0
	for _, term := range terms {
		i++
		fmt.Printf("%d) %s\n", i, term)
	}
	fmt.Println("*) All terms")

	var allTerms bool
	var termIndex int
	for {
		fmt.Print("Type number or *: ")
		var termInput string
		if _, err = fmt.Scanln(&termInput); err != nil {
			continue
		}
		termInput = strings.Trim(termInput, " \t\n")
		if termInput == "*" {
			allTerms = true
			termIndex = -1
			break
		}

		if termNumber, err := strconv.Atoi(termInput); err == nil {
			if termNumber >= 1 && termNumber <= len(terms) {
				allTerms = false
				termIndex = termNumber - 1
				break
			}
		}

		fmt.Println("Invalid input")
	}

	if !allTerms {
		terms = []Term{terms[termIndex]}
	}

	fmt.Println("Downloading marks...")

	total := 0
	for _, term := range terms {
		termMap := subjectsMap[term.Value()]
		for _, subjects := range termMap {
			total = total + len(subjects)
		}
	}

	i = 1
	for _, term := range terms {
		termMap := subjectsMap[term.Value()]

		var classes []string
		for classSection, _ := range termMap {
			classes = append(classes, classSection)
		}
		sort.Strings(classes)

		for _, classSection := range classes {
			subjects := termMap[classSection]
			class, section, err := parseClassSection(classSection)
			if err != nil {
				panic(err)
			}
			for _, subject := range subjects {
				fmt.Printf("(%4d/%4d) ", i, total)
				download(term, class, section, subject)
				i++
			}
		}
	}
	fmt.Println("Done")

}

func download(term Term, class, section, subject string) {
	subject = strings.Replace(subject, "/", "_", -1)
	class = strings.Replace(class, "/", "_", -1)
	section = strings.Replace(section, "/", "_", -1)

	id := fmt.Sprintf("%15s %7s %s %22s", term, class, section, subject)

	filedir := filepath.Join(".", "Marks", term.String(), subject)
	filename := fmt.Sprintf("%s-%s%s.csv", subject, class, section)
	file := filepath.Join(filedir, filename)

	_, err := os.Stat(file)
	if !os.IsNotExist(err) {
		fmt.Printf("%s: exists\n", id)
		return
	}

	downloadUrl, err := url.Parse("https://creativity-private-school-2015.appspot.com/marks/export")
	if err != nil {
		panic(err)
	}

	query := downloadUrl.Query()
	query.Set("Term", term.Value())
	query.Set("ClassSection", fmt.Sprintf("%s|%s", class, section))
	query.Set("Subject", subject)

	downloadUrl.RawQuery = query.Encode()

	resp, err := http.Get(downloadUrl.String())
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		panic(resp)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	nLines := bytes.Count(body, []byte("\n"))
	if nLines <= 2 {
		// Not Applicable
		fmt.Printf("%s: skipped\n", id)
		return
	}

	err = os.MkdirAll(filedir, os.ModePerm)
	if err != nil {
		panic(err)
	}

	tempFile, err := ioutil.TempFile(filedir, "tmp")
	if err != nil {
		panic(err)
	}
	tempFilepath := tempFile.Name()
	err = tempFile.Close()
	if err != nil {
		panic(err)
	}

	err = ioutil.WriteFile(tempFilepath, body, os.ModePerm)
	if err != nil {
		panic(err)
	}

	err = os.Rename(tempFilepath, file)
	if err != nil {
		panic(err)
	}

	fmt.Printf("%s: saved to %s\n", id, file)
}
