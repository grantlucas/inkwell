package weather

// ConditionFromWMO maps a WMO weather interpretation code to a Condition.
// See https://open-meteo.com/en/docs for the full code table.
func ConditionFromWMO(code int) Condition {
	switch {
	case code == 0:
		return Clear
	case code >= 1 && code <= 2:
		return PartlyCloudy
	case code == 3:
		return Cloudy
	case code >= 45 && code <= 48:
		return Fog
	case code >= 51 && code <= 57:
		return Drizzle
	case code >= 61 && code <= 67:
		return Rain
	case code >= 71 && code <= 77:
		return Snow
	case code >= 80 && code <= 82:
		return Rain
	case code >= 85 && code <= 86:
		return Snow
	case code >= 95 && code <= 99:
		return Thunderstorm
	default:
		return Clear
	}
}

// Label returns a short uppercase label for the condition, matching the
// design system's weather label format.
func (c Condition) Label() string {
	switch c {
	case Clear:
		return "SUNNY"
	case PartlyCloudy:
		return "P.CLOUDY"
	case Cloudy:
		return "CLOUDY"
	case Rain:
		return "RAIN"
	case Snow:
		return "SNOW"
	case Thunderstorm:
		return "T.STORM"
	case Fog:
		return "FOG"
	case Drizzle:
		return "DRIZZLE"
	default:
		return ""
	}
}
