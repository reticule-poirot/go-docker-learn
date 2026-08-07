package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
)

var port int

func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

func ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ready"))
}

func home(w http.ResponseWriter, r *http.Request) {
	hostname := os.Getenv("HOSTNAME")
	log.Printf("Hello, I'm %v", hostname)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprintln("Hello, I'm", hostname, "!")))
}

func main() {

	flag.IntVar(&port, "port", 8000, "an int")
	flag.Parse()

	log.Printf("Port: %v", port)

	router := mux.NewRouter()
	router.HandleFunc("/", home).Methods(http.MethodGet)
	router.HandleFunc("/health", health).Methods(http.MethodGet)
	router.HandleFunc("/ready", ready).Methods(http.MethodGet)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(port), router))
}
