package main

import (
	"encoding/json"
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
