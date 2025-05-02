package main

import (
	//"github.com/freiheit-com/kuberpult/services/test-service/pub"
	"log"
	"net/http"
)

type MyServer struct{}

func (s *MyServer) GetClient(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("hello world get client"))
}

func (s *MyServer) UpdateClient(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	w.Write([]byte("hello world update client"))
}

func main() {
	//// Create a new Echo instance
	//e := echo.New()
	//
	//// Add middleware (optional)
	//e.Use(middleware.Logger())
	//e.Use(middleware.Recover())
	//
	//// Create an instance of your server implementation
	//server := &MyServer{}
	//
	//// Use the generated RegisterHandlers to bind your implementation to the Echo router
	//myapi2.R(e, server)
	//
	//// Start the Echo server on port 8080
	//log.Println("Starting server on :8080")
	//log.Fatal(e.Start(":8080"))
	//

	// Create an instance of your server implementation
	server := &MyServer{}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// Use the generated HandlerFromMux to attach routes to the mux
	myapi2.HandlerFromMux(server, mux)

	// Start the HTTP server
	log.Println("Starting server on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
