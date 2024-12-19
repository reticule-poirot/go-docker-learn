package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"os"
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", HomeHandler)
	log.Fatal(http.ListenAndServe(":8000", router))
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
        hostname := os.Getenv("HOSTNAME")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintln("Hello, I'm", hostname, "!")))
}
