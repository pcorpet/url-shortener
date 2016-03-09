package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

func main() {
	s := &server{
		ShortURLPrefix: os.Getenv("SHORT_URL_PREFIX"),
		DB:             &mongoDatabase{URL: os.Getenv("MONGODB_URL")},
	}

	r := mux.NewRouter()
	r.HandleFunc("/"+internalPagesPrefix+"/save", s.Save).Methods("POST")
	r.HandleFunc("/{name}{folder:(/.*)?}", s.Load)
	r.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "public/index.html")
	})

	port := "5000"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	log.Fatal(http.ListenAndServe(":"+port, handlers.LoggingHandler(os.Stdout, r)))
}
