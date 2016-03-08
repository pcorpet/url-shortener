package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

func TestServerLoad(t *testing.T) {
	tests := []struct {
		desc              string
		request           string
		loadURL           string
		loadURLError      error
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
			expectRedirect:    "/.#/?error=No+such+URL+yet.+Feel+free+to+add+one.&name=wiki",
		},
		{
			desc:              "DB load error",
			request:           "http://go/wiki",
			loadURLError:      errors.New("Could not connect to DB"),
			expectLoadedNames: []string{"wiki"},
			expectCode:        http.StatusInternalServerError,
		},
	}

	for _, test := range tests {
		var loadedNames []string
		s := &server{
			DB: &stubDB{
				loadURL: func(name string) (string, error) {
					loadedNames = append(loadedNames, name)
					return test.loadURL, test.loadURLError
				},
			},
		}

		r := mux.NewRouter()
		r.HandleFunc("/{name}{folder:(/.*)?}", s.Load)

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
			DB: &stubDB{
				saveURL: func(name string, url string) error {
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

type stubDB struct {
	loadURL func(string) (string, error)
	saveURL func(string, string) error
}

func (s stubDB) LoadURL(name string) (string, error) {
	if s.loadURL == nil {
		return "", fmt.Errorf("LoadURL(%q) called", name)
	}
	return s.loadURL(name)
}

func (s stubDB) SaveURL(name string, url string) error {
	if s.saveURL == nil {
		return fmt.Errorf("SaveURL(%q, %q) called", name, url)
	}
	return s.saveURL(name, url)
}
