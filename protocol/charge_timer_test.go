package protocol

import "testing"

func TestParseChargeTimerState(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{name: "low nibble one", data: []byte{0x01}, want: "on"},
		{name: "high nibble one low nibble one", data: []byte{0x11}, want: "on"},
		{name: "high nibble two low nibble one", data: []byte{0x21}, want: "on"},
		{name: "low nibble two", data: []byte{0x02}, want: "off"},
		{name: "high nibble one low nibble two", data: []byte{0x12}, want: "off"},
		{name: "low nibble three", data: []byte{0x03}, want: "off"},
		{name: "low nibble fifteen", data: []byte{0x0f}, want: "off"},
		{name: "low nibble zero", data: []byte{0x00}, want: "unknown"},
		{name: "high nibble only", data: []byte{0x10}, want: "unknown"},
		{name: "nil data", data: nil, want: "unknown"},
		{name: "empty data", data: []byte{}, want: "unknown"},
		{name: "extra bytes uses first byte", data: []byte{0x11, 0x99}, want: "on"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParseChargeTimerState(tc.data); got != tc.want {
				t.Fatalf("ParseChargeTimerState(%v) = %q, want %q", tc.data, got, tc.want)
			}
		})
	}
}
