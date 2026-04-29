package weather

import "testing"

func TestConditionFromWMO(t *testing.T) {
	tests := []struct {
		code int
		want Condition
	}{
		{0, Clear},
		{1, PartlyCloudy},
		{2, PartlyCloudy},
		{3, Cloudy},
		{45, Fog},
		{48, Fog},
		{51, Drizzle},
		{53, Drizzle},
		{55, Drizzle},
		{56, Drizzle},
		{57, Drizzle},
		{61, Rain},
		{63, Rain},
		{65, Rain},
		{66, Rain},
		{67, Rain},
		{71, Snow},
		{73, Snow},
		{75, Snow},
		{77, Snow},
		{80, Rain},
		{81, Rain},
		{82, Rain},
		{85, Snow},
		{86, Snow},
		{95, Thunderstorm},
		{96, Thunderstorm},
		{99, Thunderstorm},
		{-1, Clear},
		{999, Clear},
	}
	for _, tt := range tests {
		got := ConditionFromWMO(tt.code)
		if got != tt.want {
			t.Errorf("ConditionFromWMO(%d) = %v, want %v", tt.code, got, tt.want)
		}
	}
}

func TestCondition_Label(t *testing.T) {
	tests := []struct {
		cond Condition
		want string
	}{
		{Clear, "SUNNY"},
		{PartlyCloudy, "P.CLOUDY"},
		{Cloudy, "CLOUDY"},
		{Rain, "RAIN"},
		{Snow, "SNOW"},
		{Thunderstorm, "T.STORM"},
		{Fog, "FOG"},
		{Drizzle, "DRIZZLE"},
		{Condition(99), ""},
	}
	for _, tt := range tests {
		got := tt.cond.Label()
		if got != tt.want {
			t.Errorf("Condition(%d).Label() = %q, want %q", tt.cond, got, tt.want)
		}
	}
}
