package main

import (
	"net/http"
	"fmt"
	"os"
	"log"
	"time"
	"io/ioutil"
)

var startedAt = time.Now()

func main() {
	http.HandleFunc("/healthz", Healthz)
	http.HandleFunc("/secret", Secret)
	http.HandleFunc("/configmap", ConfigMap)
	http.HandleFunc("/", Hello)
	http.ListenAndServe(":8080", nil)
}

func Hello(w http.ResponseWriter, r *http.Request) {

	name := os.Getenv("NAME")
	age := os.Getenv("AGE")

	fmt.Fprintf(w, "Hello, I'am %s. I'am %s years old.", name, age)
}

func ConfigMap(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadFile("/go/myfamily/family.txt")
	if err != nil {
		log.Fatalf("Error reading file: ", err)
	}
	fmt.Fprintf(w, "My Family: %s.", string(data))
}

func Secret(w http.ResponseWriter, r *http.Request) {
	user := os.Getenv("USER")
	password := os.Getenv("PASSWORD")

	fmt.Fprintf(w, "User: %s, Password: %s.", user, password)
}

func Healthz(w http.ResponseWriter, r *http.Request) {
	durantion := time.Since(startedAt)
	
	if durantion.Seconds() < 10 || durantion.Seconds() > 30 {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("Duration: %v", durantion.Seconds())))	
	} else {
		w.WriteHeader(200)
		w.Write([]byte("ok"))
	}
}