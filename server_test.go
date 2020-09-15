package main

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
)

type fakeClock struct { now time.Time }
func (f fakeClock) Now() time.Time { return f.now }

func TestServerList(t *testing.T) {
	tests := []struct {
		desc                string
		listURLs            []namedURL
		listURLsError       error
		forwardedUser       string
		expectListURLsCalls int
		expectCode          int
		expectBody          string
	}{
		{
			desc: "Simple list",
			listURLs: []namedURL{
				{
					Name:   "wiki",
					URL:    "http://github.com/bayesimpact/wiki",
					Owners: []string{"pascal@bayesimpact.org"},
				},
				{
					Name:              "google",
					URL:               "http://www.google.com",
					Owners:            []string{},
					ShouldExpandDates: true,
				},
			},
			expectCode: http.StatusOK,
			expectBody: `{"urls":[` +
				`{"name":"wiki","url":"http://github.com/bayesimpact/wiki","owners":["pascal@bayesimpact.org"],"shouldExpandDates":false},` +
				`{"name":"google","url":"http://www.google.com","owners":[],"shouldExpandDates":true}` +
				`]}`,
			expectListURLsCalls: 1,
		},
		{
			desc:                "No URLs",
			expectCode:          http.StatusOK,
			expectBody:          `{"urls":[]}`,
			expectListURLsCalls: 1,
		},
		{
			desc:                "Error",
			expectCode:          http.StatusInternalServerError,
			listURLsError:       errors.New("Oh oh"),
			expectBody:          `{"error":"Oh oh"}` + "\n",
			expectListURLsCalls: 1,
		},
		{
			desc:                "User",
			forwardedUser:       "lascap",
			expectCode:          http.StatusOK,
			expectBody:          `{"urls":[],"user":"lascap"}`,
			expectListURLsCalls: 1,
		},
		{
			desc:                "Super User",
			forwardedUser:       "SUPER USER",
			expectCode:          http.StatusOK,
			expectBody:          `{"superUser":true,"urls":[],"user":"SUPER USER"}`,
			expectListURLsCalls: 1,
		},
	}

	for _, test := range tests {
		listURLsCalls := 0
		s := &server{
			DB: &stubDB{
				listURLs: func() ([]namedURL, error) {
					listURLsCalls += 1
					return test.listURLs, test.listURLsError
				},
			},
			SuperUser: map[string]bool{"SUPER USER": true},
			Clock: realClock{},
		}

		r := mux.NewRouter()
		r.HandleFunc("/list", s.List).Methods("POST")

		response := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "http://go/list", nil)
		if err != nil {
			t.Errorf("%s: test setup error, impossible to create request: %v", test.desc, err)
			continue
		}
		if test.forwardedUser != "" {
			request.Header.Set("X-Forwarded-User", test.forwardedUser)
		}

		r.ServeHTTP(response, request)

		if got, want := response.Code, test.expectCode; got != want {
			t.Errorf("%s: s.List(...) had response code %d, want %d\n%v", test.desc, got, want, response)
		}

		if got, want := response.Body.String(), test.expectBody; got != want {
			t.Errorf("%s: s.List(...) returned a body with %q, want %q", test.desc, got, want)
		}

		if got, want := listURLsCalls, test.expectListURLsCalls; got != want {
			t.Errorf("%s: s.List(...) did %d call(s) to db.ListURLs, want %d", test.desc, got, want)
		}
	}
}

func TestServerLoad(t *testing.T) {
	tests := []struct {
		desc              string
		request           string
		loadURL           string
		loadURLError      error
		shouldExpandDates bool
		expectLoadedNames []string
		expectCode        int
		expectRedirect    string
	}{
		{
			desc:              "Successful load",
			request:           "http://go/wiki",
			loadURL:           "http://github.com/bayesimpact/wiki",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://github.com/bayesimpact/wiki",
		},
		{
			desc:              "Forward query string",
			request:           "http://go/wiki?foo=bar",
			loadURL:           "http://github.com/bayesimpact/wiki",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://github.com/bayesimpact/wiki?foo=bar",
		},
		{
			desc:              "Drop query string if the stored URL already has one",
			request:           "http://go/wiki?foo=bar",
			loadURL:           "http://github.com/bayesimpact/wiki?go",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://github.com/bayesimpact/wiki?go",
		},
		{
			desc:              "Forward subfolder",
			request:           "http://go/wiki/New-Hire-Resources",
			loadURL:           "http://github.com/bayesimpact/wiki",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://github.com/bayesimpact/wiki/New-Hire-Resources",
		},
		{
			desc:              "Forward subfolder when URL ends with a /",
			request:           "http://go/wiki/New-Hire-Resources",
			loadURL:           "http://en.wikipedia.org/",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://en.wikipedia.org/New-Hire-Resources",
		},
		{
			desc:              "Forward subfolder and query string",
			request:           "http://go/wiki/New-Hire-Resources?foo=bar",
			loadURL:           "http://github.com/bayesimpact/wiki",
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusMovedPermanently,
			expectRedirect:    "http://github.com/bayesimpact/wiki/New-Hire-Resources?foo=bar",
		},
		{
			desc:              "Short URL not found",
			request:           "http://go/wiki",
			loadURLError:      NotFoundError{"wiki"},
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusFound,
			expectRedirect:    "/#/?error=No+such+URL+yet.+Feel+free+to+add+one.&name=wiki",
		},
		{
			desc:              "Short URL not found but uses subfolder",
			request:           "http://go/wiki/settings",
			loadURLError:      NotFoundError{"wiki"},
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusFound,
			expectRedirect:    "/#/?error=No+such+URL+yet.+Feel+free+to+add+one.&name=wiki",
		},
		{
			desc:              "DB load error",
			request:           "http://go/wiki",
			loadURLError:      errors.New("Could not connect to DB"),
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusInternalServerError,
		},
		{
			desc:              "Expand dates",
			request:           "http://go/okr",
			loadURL:           "/okr-2006-01",
			shouldExpandDates: true,
			expectLoadedNames: []string{"okr"},
			expectCode:        http.StatusFound,
			// The test is meant to be run on fake date "2020-09-03".
			expectRedirect:    "/okr-2020-09",
		},
	}

	testTime, err := time.Parse("2006-01-02", "2020-09-03")
	if err != nil {
		t.Errorf("Could not parse the testing time: %v", err)
		return
	}

	for _, test := range tests {
		var loadedNames []string
		s := &server{
			Clock: fakeClock{now: testTime},
			DB: &stubDB{
				loadURL: func(name string) (namedURL, error) {
					loadedNames = append(loadedNames, name)
					return namedURL{URL: test.loadURL, ShouldExpandDates: test.shouldExpandDates}, test.loadURLError
				},
			},
		}

		r := mux.NewRouter()
		r.HandleFunc("/{name}{folder:(?:/.*)?}", s.Load)

		response := httptest.NewRecorder()
		request, err := http.NewRequest("GET", test.request, nil)
		if err != nil {
			t.Errorf("%s: test setup error, impossible to create request: %v", test.desc, err)
			continue
		}

		r.ServeHTTP(response, request)

		if got, want := response.Code, test.expectCode; got != want {
			t.Errorf("%s: s.Load(...) had response code %d, want %d\n%v", test.desc, got, want, response)
		}

		if !reflect.DeepEqual(loadedNames, test.expectLoadedNames) {
			t.Errorf("%s: s.Load(...) tried to load %q, wanted %q", test.desc, loadedNames, test.expectLoadedNames)
		}

		if want := test.expectRedirect; want != "" {
			if got := response.HeaderMap.Get("Location"); got != want {
				t.Errorf("%s: s.Load(...) redirected to %q, want %q", test.desc, got, want)
			}
		}
	}
}

func TestSave(t *testing.T) {
	tests := []struct {
		desc            string
		body            string
		saveURLError    error
		expectSavedURLs map[string]string
		expectCode      int
		expectBody      string
	}{
		{
			desc:            "Successful save",
			body:            `{"name": "wiki", "url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusOK,
			expectSavedURLs: map[string]string{"wiki": "http://github.com/bayesimpact/wiki"},
			expectBody:      `{"name":"wiki"}`,
		},
		{
			desc:            "OK with extra vars",
			body:            `{"name": "wiki", "url": "http://github.com/bayesimpact/wiki", "other": "foo"}`,
			expectCode:      http.StatusOK,
			expectSavedURLs: map[string]string{"wiki": "http://github.com/bayesimpact/wiki"},
			expectBody:      `{"name":"wiki"}`,
		},
		{
			desc:            "Missing name",
			body:            `{"url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Missing name"}` + "\n",
		},
		{
			desc:            "Empty name",
			body:            `{"name": "", "url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Missing name"}` + "\n",
		},
		{
			desc:            "Name with a slash",
			body:            `{"name": "bayesimpact/wiki", "url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Name (\"bayesimpact/wiki\") contains an illegal character: \"/?#\""}` + "\n",
		},
		{
			desc:            "Reserved name",
			body:            `{"name": "_", "url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Name (\"_\") is reserved for the shortener use"}` + "\n",
		},
		{
			desc:            "Successful save when name starts with reserved prefix",
			body:            `{"name": "_wiki", "url": "http://github.com/bayesimpact/wiki"}`,
			expectCode:      http.StatusOK,
			expectSavedURLs: map[string]string{"_wiki": "http://github.com/bayesimpact/wiki"},
			expectBody:      `{"name":"_wiki"}`,
		},
		{
			desc:            "Missing URL",
			body:            `{"name": "wiki"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Missing URL for \"wiki\""}` + "\n",
		},
		{
			desc:            "Empty URL",
			body:            `{"name": "wiki", "url": ""}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Missing URL for \"wiki\""}` + "\n",
		},
		{
			desc:            "Unparseable json",
			body:            `{--}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Unable to parse json"}` + "\n",
		},
		{
			desc:            "DB save error",
			body:            `{"name": "wiki", "url": "http://github.com/bayesimpact/wiki"}`,
			saveURLError:    errors.New("Could not connect to DB"),
			expectCode:      http.StatusInternalServerError,
			expectSavedURLs: map[string]string{"wiki": "http://github.com/bayesimpact/wiki"},
			expectBody:      `{"error":"Could not connect to DB"}` + "\n",
		},
		{
			desc:            "Not an URL",
			body:            `{"name": "wiki", "url": ":^@$"}`,
			expectCode:      http.StatusBadRequest,
			expectSavedURLs: map[string]string{},
			expectBody:      `{"error":"Not a valid URL: \":^@$\"."}` + "\n",
		},
	}

	for _, test := range tests {
		savedURLs := map[string]string{}
		s := &server{
			Clock: realClock{},
			DB: &stubDB{
				saveURL: func(name string, url string, owners []string, shouldExpandDates bool) error {
					savedURLs[name] = url
					return test.saveURLError
				},
			},
		}

		r := mux.NewRouter()
		r.HandleFunc("/save", s.Save).Methods("POST")

		response := httptest.NewRecorder()
		request, err := http.NewRequest("POST", "http://go/save", nil)
		if err != nil {
			t.Errorf("%s: test setup error, impossible to create request: %v", test.desc, err)
			continue
		}
		request.Body = ioutil.NopCloser(strings.NewReader(test.body))

		r.ServeHTTP(response, request)

		if got, want := response.Code, test.expectCode; got != want {
			t.Errorf("%s: s.Save(...) had response code %d, want %d\n%v", test.desc, got, want, response)
			continue
		}

		if !reflect.DeepEqual(savedURLs, test.expectSavedURLs) {
			t.Errorf("%s: s.Save(...) saved these URLs\n%v\nbut wanted those\n%v", test.desc, savedURLs, test.expectSavedURLs)
		}

		if got, want := response.Body.String(), test.expectBody; got != want {
			t.Errorf("%s: s.Save(...) returned a body with %q, want %q", test.desc, got, want)
		}
	}
}

func TestDelete(t *testing.T) {
	tests := []struct {
		desc              string
		request           string
		forwardedUser     string
		deleteURLError    error
		expectDeletedURLs []string
		expectCode        int
		expectBody        string
	}{
		{
			desc:              "Typical delete",
			request:           "/wiki",
			forwardedUser:     "lascap",
			expectDeletedURLs: []string{"wiki", "lascap"},
			expectCode:        http.StatusOK,
			expectBody:        `{"success":true}`,
		},
		{
			desc:       "Missing user",
			request:    "/wiki",
			expectCode: http.StatusUnauthorized,
			expectBody: `{"error":"Request with no user"}` + "\n",
		},
		{
			desc:              "DB error",
			request:           "/wiki",
			forwardedUser:     "lascap",
			deleteURLError:    errors.New("failure!"),
			expectDeletedURLs: []string{"wiki", "lascap"},
			expectCode:        http.StatusInternalServerError,
			expectBody:        `{"error":"failure!"}` + "\n",
		},
		{
			desc:              "Super user",
			request:           "/wiki",
			forwardedUser:     "SUPER USER",
			expectDeletedURLs: []string{"wiki", ""},
			expectCode:        http.StatusOK,
			expectBody:        `{"success":true}`,
		},
	}

	for _, test := range tests {
		var deletedURLs []string
		s := &server{
			DB: &stubDB{
				deleteURL: func(name string, url string) error {
					deletedURLs = append(deletedURLs, name, url)
					return test.deleteURLError
				},
			},
			SuperUser: map[string]bool{"SUPER USER": true},
		}

		r := mux.NewRouter()
		r.HandleFunc("/{name}", s.Delete).Methods("DELETE")

		response := httptest.NewRecorder()
		request, err := http.NewRequest("DELETE", fmt.Sprintf("http://go%s", test.request), nil)
		if err != nil {
			t.Errorf("%s: test setup error, impossible to create request: %v", test.desc, err)
			continue
		}
		if test.forwardedUser != "" {
			request.Header.Set("X-Forwarded-User", test.forwardedUser)
		}

		r.ServeHTTP(response, request)

		if got, want := response.Code, test.expectCode; got != want {
			t.Errorf("%s: s.Delete(...) had response code %d, want %d\n%v", test.desc, got, want, response)
			continue
		}

		if !reflect.DeepEqual(deletedURLs, test.expectDeletedURLs) {
			t.Errorf("%s: s.Delete(...) deleted these URLs\n%v\nbut wanted those\n%v", test.desc, deletedURLs, test.expectDeletedURLs)
		}

		if got, want := response.Body.String(), test.expectBody; got != want {
			t.Errorf("%s: s.Delete(...) returned a body with %q, want %q", test.desc, got, want)
		}
	}
}

type stubDB struct {
	deleteURL func(string, string) error
	listURLs  func() ([]namedURL, error)
	loadURL   func(string) (namedURL, error)
	saveURL   func(string, string, []string, bool) error
}

func (s stubDB) DeleteURL(ctx context.Context, name, user string) error {
	if s.deleteURL == nil {
		return errors.New("DeleteURL called")
	}
	return s.deleteURL(name, user)
}

func (s stubDB) ListURLs(ctx context.Context) ([]namedURL, error) {
	if s.listURLs == nil {
		return nil, errors.New("ListURLs called")
	}
	return s.listURLs()
}

func (s stubDB) LoadURL(ctx context.Context, name string) (namedURL, error) {
	if s.loadURL == nil {
		return namedURL{}, fmt.Errorf("LoadURL(%q) called", name)
	}
	return s.loadURL(name)
}

func (s stubDB) SaveURL(ctx context.Context, name string, url string, owners []string, shouldExpandDates bool) error {
	if s.saveURL == nil {
		return fmt.Errorf("SaveURL(%q, %q, %v, %t) called", name, url, owners, shouldExpandDates)
	}
	return s.saveURL(name, url, owners, shouldExpandDates)
}
