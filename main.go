package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

type realClock struct {}
func (realClock) Now() time.Time { return time.Now() }

func main() {
	dbName := os.Getenv("MONGODB_DB_NAME")
	if dbName == "" {
		dbName = "url-shortener"
	}
	collectionName := os.Getenv("MONGODB_COLLECTION_NAME")
	if collectionName == "" {
		collectionName = "shortURL"
	}
	s := &server{
		ShortURLPrefix: os.Getenv("SHORT_URL_PREFIX"),
		DB:             &mongoDatabase{
			URL: os.Getenv("MONGODB_URL"),
			DBName: dbName,
			CollectionName: collectionName,
		},
		Clock: realClock{},
	}

	if superUsers := strings.TrimSpace(os.Getenv("SUPER_USERS")); superUsers != "" {
		s.SuperUser = map[string]bool{}
		for _, superUser := range strings.Split(superUsers, ",") {
			s.SuperUser[strings.TrimSpace(superUser)] = true
		}
	}

	r := mux.NewRouter()
	r.HandleFunc("/"+internalPagesPrefix+"/list", s.List).Methods("POST")
	r.HandleFunc("/"+internalPagesPrefix+"/save", s.Save).Methods("POST")
	r.HandleFunc("/"+internalPagesPrefix+"/{name}", s.Delete).Methods("DELETE")
	r.HandleFunc("/{name}{folder:(?:/.*)?}", s.Load)
	r.HandleFunc("/", func(response http.ResponseWriter, request *http.Request) {
		http.ServeFile(response, request, "public/index.html")
	})

	port := "5000"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	log.Fatal(http.ListenAndServe(":"+port, handlers.LoggingHandler(os.Stdout, r)))
}
