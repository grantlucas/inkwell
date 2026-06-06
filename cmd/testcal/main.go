package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/grantlucas/inkwell/internal/inkwell/calendar/testcal"
)

func main() {
	http.HandleFunc("/test.ics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/calendar")
		if _, err := fmt.Fprint(w, testcal.Generate(time.Now())); err != nil {
			log.Printf("write /test.ics response: %v", err)
		}
	})
	fmt.Println("Serving test calendar on :9999/test.ics")
	if err := http.ListenAndServe(":9999", nil); err != nil {
		log.Fatalf("listen on :9999: %v", err)
	}
}
