// +build !appengine

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var cookieValue = "REPLACE"

var subjects = []string{
	"English",
	"Speech and Drama",
	"Math",
	"Social Studies",
	"Science",
	"Computer",
	"Arabic",
	"Citizenship",
	"Social Studies Arabic",
	"UCMAS",
	"Islamic Studies",
	"PE",
	"Biology",
	"Physics",
	"Chemistry",
	"Economics",
	"Accounting",
	"Business Studies",
	"Environmental Science",
	"Ecology",
	"Arabic 102",
	"Arabic 201",
	"Arabic 101",
	"Arabic 202",
	"Arabic 301",
	"Arabic 302",
	"Islamic Studies 101",
	"Islamic Studies 301",
	"Islamic Studies 201",
	"Islamic Studies 103",
	"Islamic Studies 104",
}

var extraSubjects = []string{
	"Remarks",
	"Behavior",
}

var classes = []struct {
	class      string
	maxSection rune
}{
	{"1A", 'A'},
	{"1", 'D'},
	{"2", 'D'},
	{"3", 'D'},
	{"4", 'C'},
	{"5", 'B'},
	{"6", 'C'},
	{"7", 'B'},
	{"8", 'B'},
	{"9", 'A'},
	{"9SCI", 'B'},
	{"9COM", 'B'},
	{"10SCI", 'B'},
	{"10COM", 'B'},
	{"11SCI", 'A'},
	{"11COM", 'A'},
	{"12SCI", 'A'},
	{"12COM", 'A'},
	{"PREKG", 'A'},
	{"KG1", 'C'},
	{"KG2", 'D'},
	{"SN", 'A'},
}

// Copied from gradingsystems.go
type termType int

const (
	Quarter termType = iota + 1
	Semester
	EndOfYear
	EndOfYearGpa
)

var termStrings = map[termType]string{
	Quarter:      "Quarter",
	Semester:     "Semester",
	EndOfYear:    "End of Year",
	EndOfYearGpa: "End of Year (GPA)",
}

type Term struct {
	Typ termType
	N   int
}

var terms = []Term{
	{Quarter, 1},
	{Quarter, 2},
	{Semester, 1},
	{Quarter, 3},
	{Quarter, 4},
	{Semester, 2},
	{EndOfYear, 0},
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

func main() {
	var err error
	http.DefaultClient.Jar, err = cookiejar.New(nil)
	if err != nil {
		panic(err)
	}

	baseUrl, err := url.Parse("https://creativity-private-school-2015.appspot.com/")
	if err != nil {
		panic(err)
	}
	http.DefaultClient.Jar.SetCookies(baseUrl, []*http.Cookie{
		{
			Name:  "SACSID",
			Value: cookieValue,
			Path:     "/",
			Secure:   true,
			HttpOnly: true,
		},
	})

	http.DefaultClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Host != via[len(via)-1].URL.Host {
			return fmt.Errorf("Redirecting to new host: %s", req.URL.Host)
		}
		return nil
	}

	var allSubjects []string
	allSubjects = append(allSubjects, subjects...)
	allSubjects = append(allSubjects, extraSubjects...)

	for _, term := range terms {
		for _, class := range classes {
			if class.maxSection < 'A' ||
				class.maxSection > 'Z' {
				fmt.Printf("Invalid maxSection: %#v", class)
			}

			for section := 'A'; section <= class.maxSection; section++ {
				for _, subject := range allSubjects {
					download(term, class.class, section, subject)
				}
			}
		}
	}

}

func download(term Term, class string, section rune, subject string) {
	id := fmt.Sprintf("%15s %7s %c %22s", term, class, section, subject)

	filedir := filepath.Join(".", "Marks", term.String(), subject)
	filename := fmt.Sprintf("%s-%s%c.csv", subject, class, section)
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
	query.Set("ClassSection", fmt.Sprintf("%s|%c", class, section))
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
