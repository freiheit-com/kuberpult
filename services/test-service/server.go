package main

import (
	"github.com/freiheit-com/kuberpult/services/test-service/myapi2"
	"github.com/labstack/echo/v4"
	"log"
	"net/http"
)

type MyServer struct{}

func (s *MyServer) GetUsers(ctx echo.Context, params string) (string, error) {
	return "holla", nil
}

func (s *MyServer) CreateUser(ctx echo.Context, newUser string) (string, error) {
	return "42", nil
}

//func (s *MyServer) GetClient(ctx echo.Context) error {
//	//TODO implement me
//	panic("implement me")
//}
//
//func (s *MyServer) UpdateClient(ctx echo.Context) error {
//	//TODO implement me
//	panic("implement me")
//}

func (s *MyServer) GetClient(w http.ResponseWriter, r *http.Request) {
	panic("implement me")
}

func (s *MyServer) UpdateClient(w http.ResponseWriter, r *http.Request) {
	panic("implement me")
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
