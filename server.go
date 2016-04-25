package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"

	"github.com/gorilla/mux"

	neturl "net/url"
)

// internalPagesPrefix is a prefix that is reserved (cannot be used as a
// shortened URL name) for the pages and method of the shortener itself.
const internalPagesPrefix = "_"

type server struct {
	// ShortURLPrefix is an optional prefix to return even shorter URLs than
	// using the request's hostname and path.
	ShortURLPrefix string

	DB database
}

// illegalChars is a string containing all characters that are illegal in short
// URL names. They are illegal because they have a special meaning when using
// the short URL link.
const illegalChars = "/?#"

func (s server) Save(response http.ResponseWriter, request *http.Request) {
	decoder := json.NewDecoder(request.Body)
	var data namedURL
	if err := decoder.Decode(&data); err != nil {
		http.Error(response, `{"error":"Unable to parse json"}`, http.StatusBadRequest)
		return
	}

	if data.Name == "" {
		http.Error(response, `{"error":"Missing name"}`, http.StatusBadRequest)
		return
	}

	if data.Name == internalPagesPrefix {
		reply := map[string]string{"error": fmt.Sprintf("Name (%q) is reserved for the shortener use", internalPagesPrefix)}
		if jsonData, ok := marshalJson(response, reply); ok {
			http.Error(response, string(jsonData), http.StatusBadRequest)
		}
		return
	}

	if strings.ContainsAny(data.Name, illegalChars) {
		if jsonData, ok := marshalJson(response, map[string]string{"error": fmt.Sprintf("Name (%q) contains an illegal character: %q", data.Name, illegalChars)}); ok {
			http.Error(response, string(jsonData), http.StatusBadRequest)
		}
		return
	}

	if data.URL == "" {
		if jsonData, ok := marshalJson(response, map[string]string{"error": fmt.Sprintf("Missing URL for %q", data.Name)}); ok {
			http.Error(response, string(jsonData), http.StatusBadRequest)
		}
		return
	}

	if _, err := neturl.Parse(data.URL); err != nil {
		if jsonData, ok := marshalJson(response, map[string]string{"error": fmt.Sprintf("Not a valid URL: %q.", data.URL)}); ok {
			http.Error(response, string(jsonData), http.StatusBadRequest)
		}
		return
	}

	if err := s.DB.SaveURL(data.Name, data.URL); err != nil {
		if jsonData, ok := marshalJson(response, map[string]string{"error": err.Error()}); ok {
			http.Error(response, string(jsonData), http.StatusInternalServerError)
		}
		return
	}

	resp := map[string]string{"name": data.Name}
	if s.ShortURLPrefix != "" {
		resp["url"] = s.ShortURLPrefix
	}
	if jsonData, ok := marshalJson(response, resp); ok {
		response.Write(jsonData)
	}
}

func (s server) Load(response http.ResponseWriter, request *http.Request) {
	name := mux.Vars(request)["name"]

	url, err := s.DB.LoadURL(name)
	if err != nil {
		if _, ok := err.(NotFoundError); ok {
			q := neturl.Values{}
			q.Add("name", name)
			q.Add("error", "No such URL yet. Feel free to add one.")
			http.Redirect(response, request, "/#/?"+q.Encode(), http.StatusFound)
			return
		}

		if jsonData, ok := marshalJson(response, map[string]string{"error": err.Error()}); ok {
			http.Error(response, string(jsonData), http.StatusInternalServerError)
		}
		return
	}

	var u *neturl.URL
	if u, err = neturl.Parse(url); err != nil {
		http.Redirect(response, request, url, http.StatusMovedPermanently)
		return
	}

	var tinkered bool

	if folder := mux.Vars(request)["folder"]; folder != "" {
		u.Path = path.Join(u.Path, folder)
		tinkered = true
	}

	if q := request.URL.RawQuery; q != "" && u.RawQuery == "" {
		u.RawQuery = q
		tinkered = true
	}

	if tinkered {
		url = u.String()
	}

	http.Redirect(response, request, url, http.StatusMovedPermanently)
}

func (s server) List(response http.ResponseWriter, request *http.Request) {
	urls, err := s.DB.ListURLs()
	if err != nil {
		if jsonData, ok := marshalJson(response, map[string]string{"error": err.Error()}); ok {
			http.Error(response, string(jsonData), http.StatusInternalServerError)
		}
		return
	}

	if len(urls) == 0 {
		urls = []namedURL{}
	}

	if jsonData, ok := marshalJson(response, map[string][]namedURL{"urls": urls}); ok {
		response.Write(jsonData)
	}
}

func marshalJson(response http.ResponseWriter, reply interface{}) ([]byte, bool) {
	jsonData, err := json.Marshal(reply)
	if err != nil {
		http.Error(response, `{"error":"Unable to encode json"}`, http.StatusInternalServerError)
		return nil, false
	}
	return jsonData, true
}
