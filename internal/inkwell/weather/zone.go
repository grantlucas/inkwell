package weather

import (
	"errors"
	"io/fs"
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
// "Local" whatever it holds — so it is named the way Go resolves it: from TZ,
// then the /etc/localtime link, and UTC wherever Go gives up. A host zone that
// is set but can't be named, an /etc/localtime copied in rather than linked,
// gets "auto", the forecast location's own zone. That is only right when the
// location shares the host's zone, which is why the timezone config key
// exists.
func zoneName(zone *time.Location) string {
	if zone != time.Local {
		return zone.String()
	}
	if tz, ok := lookupEnv("TZ"); ok {
		// A leading colon is the POSIX way of saying "this is a file name".
		name, _ := zoneinfoName(strings.TrimPrefix(tz, ":"))
		if _, err := time.LoadLocation(name); name == "" || err != nil {
			return "UTC"
		}
		return name
	}
	target, err := readlink("/etc/localtime")
	if errors.Is(err, fs.ErrNotExist) {
		return "UTC"
	}
	if name, ok := zoneinfoName(target); ok && err == nil {
		return name
	}
	return "auto"
}

// zoneinfoName returns the zone named by s, which is either a zone name or a
// path into a zoneinfo tree, and whether s was such a path.
func zoneinfoName(s string) (string, bool) {
	if _, name, ok := strings.Cut(s, "zoneinfo/"); ok {
		return name, true
	}
	return s, false
}
