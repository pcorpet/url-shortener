package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	neturl "net/url"
)

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
	var data struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	}
	if err := decoder.Decode(&data); err != nil {
		http.Error(response, `{"error":"Unable to parse json"}`, http.StatusBadRequest)
		return
	}

	if data.Name == "" {
		http.Error(response, `{"error":"Missing name"}`, http.StatusBadRequest)
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
			http.Redirect(response, request, ".#/?"+q.Encode(), http.StatusFound)
		}

		if jsonData, ok := marshalJson(response, map[string]string{"error": err.Error()}); ok {
			http.Error(response, string(jsonData), http.StatusInternalServerError)
		}
		return
	}

	if folder := mux.Vars(request)["folder"]; folder != "" {
		url += folder
	}

	if q := request.URL.RawQuery; q != "" && !strings.Contains(url, "?") {
		url += "?" + q
	}

	http.Redirect(response, request, url, http.StatusMovedPermanently)
}

func marshalJson(response http.ResponseWriter, reply map[string]string) ([]byte, bool) {
	jsonData, err := json.Marshal(reply)
	if err != nil {
		http.Error(response, `{"error":"Unable to encode json"}`, http.StatusInternalServerError)
		return nil, false
	}
	return jsonData, true
}
