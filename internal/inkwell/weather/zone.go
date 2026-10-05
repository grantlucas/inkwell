package weather

import (
	"os"
	"strings"
	"time"
)

// lookupEnv and readlink are the host lookups zoneName falls back on for
// time.Local, overridden by tests to stand in for a host's configuration.
var (
	lookupEnv = os.LookupEnv
	readlink  = os.Readlink
)

// zoneName is the Open-Meteo timezone parameter for zone: its IANA name.
//
// time.Local is the one zone that doesn't carry its name — it is called
// "Local" whatever it holds — so it is resolved the way Go itself resolves it,
// from TZ and then the /etc/localtime link. A host whose zone can't be named
// that way gets "auto", the forecast location's own zone. That is only right
// when the location shares the host's zone, which is why the timezone config
// key exists.
func zoneName(zone *time.Location) string {
	if zone != time.Local {
		return zone.String()
	}
	if tz, ok := lookupEnv("TZ"); ok {
		// Go reads TZ="" as UTC, and a leading colon is the POSIX way of
		// saying "this is a file name".
		if tz = strings.TrimPrefix(tz, ":"); tz == "" {
			return "UTC"
		}
		return zoneinfoName(tz)
	}
	if target, err := readlink("/etc/localtime"); err == nil {
		if _, name, ok := strings.Cut(target, "zoneinfo/"); ok {
			return name
		}
	}
	return "auto"
}

// zoneinfoName returns a zone name given either as a name or as a path into
// a zoneinfo tree.
func zoneinfoName(s string) string {
	if _, name, ok := strings.Cut(s, "zoneinfo/"); ok {
		return name
	}
	return s
}
