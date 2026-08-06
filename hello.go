package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	router := mux.NewRouter()
	router.HandleFunc("/", HomeHandler)
	log.Fatal(http.ListenAndServe(":8000", router))
}

func HomeHandler(w http.ResponseWriter, r *http.Request) {
	hostname := os.Getenv("HOSTNAME")
	log.Printf("Hello, I'm %v", hostname)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintln("Hello, I'm", hostname, "!")))
}
