package main

import (
	"context"
	"fmt"
	"log"
	"os"

	run "google.golang.org/api/run/v1"
)

func main() {
	ctx := context.Background()

	// url := "https://run.googleapis.com/"
	// opts := []option.ClientOption{
	// 	option.WithEndpoint(url),
	// }

	runService, err := run.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}
	req := runService.Projects.Locations.Services.List("projects/fdc-standard-setup-dev-env/locations/europe-west1")
	it, err := req.Do()
	if err != nil {
		fmt.Println("Error: ", err)
		os.Exit(1)
	}
	for _, service := range it.Items {
		for _, container := range service.Spec.Template.Spec.Containers {
			fmt.Printf("%s:\t%s\n", service.Metadata.Name, container.Image)
		}
		// time.Sleep(2 * time.Second)
	}
}
