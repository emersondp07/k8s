package main

import (
	"net/http"
	"fmt"
	"os"
)

func main() {
	http.HandleFunc("/", Hello)
	http.ListenAndServe(":8080", nil)
}

func Hello(w http.ResponseWriter, r *http.Request) {

	name := os.Getenv("NAME")
	age := os.Getenv("AGE")

	fmt.Fprintf(w, "Hello, I'am %s. I'am %s years old.", name, age)
}