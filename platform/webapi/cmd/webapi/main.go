// Command webapi is the read-only HTTP gateway that serves TALOS orchestration
// state (from wt/mo/ov/ch) to the web console.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/John-Santa/talos/platform/webapi/adapter/cliexec"
	"github.com/John-Santa/talos/platform/webapi/adapter/httpapi"
	"github.com/John-Santa/talos/platform/webapi/service"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8100"
	}
	svc := service.NewGateway(cliexec.NewFromEnv())
	handler := httpapi.New(svc, os.Getenv("WEBAPI_CORS_ORIGIN"))

	addr := ":" + port
	log.Printf("talos webapi listening on %s", addr)
	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatal(err)
	}
}
