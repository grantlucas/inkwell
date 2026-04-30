package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/testcal"
)

func main() {
	http.HandleFunc("/test.ics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		fmt.Fprint(w, testcal.Generate(time.Now()))
	})
	fmt.Println("Serving test calendar on :9999/test.ics")
	http.ListenAndServe(":9999", nil)
}
