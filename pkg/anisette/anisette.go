// Package anisette produces the Apple "anisette" device-attestation headers
// required by Apple's GrandSlam Authentication (GSA) endpoints.
//
// Anisette is a set of headers (X-Apple-I-MD, X-Apple-I-MD-M, ...) that prove
// a request comes from a real, registered Apple device. The one-time password
// (X-Apple-I-MD) is minted by Apple's proprietary AOSKit/CoreADI libraries
// from a machine ID; the algorithm is not reproduced here.
//
// The Data type is a plain container consumed by package gsa. Providers back
// it with platform-specific sources:
//
//   - Windows (cgo): the local iCloud install — classic "2020" iCloud first
//     (via the Objective-C runtime), Microsoft Store iCloud as a fallback.
//   - Other platforms: public SideStore-style anisette servers.
package anisette

import (
	"context"
	"encoding/json"
	"strings"
)

// Data holds a complete set of anisette values. OTP and MachineID are the
// only strictly required pair; the rest are device/locale metadata.
type Data struct {
	OTP           string // X-Apple-I-MD
	MachineID     string // X-Apple-I-MD-M
	LocalUserUUID string // X-Apple-I-MD-LU
	DeviceID      string // X-Mme-Device-Id
	ClientInfo    string // X-MMe-Client-Info
	SerialNo      string // X-Apple-I-SRL-NO
	RoutingInfo   string // X-Apple-I-MD-RINFO
	Locale        string // X-Apple-Locale
	Timezone      string // X-Apple-I-TimeZone
	ClientTime    string // X-Apple-I-Client-Time
	UserAgent     string // User-Agent to pair with this anisette data
}

// Complete reports whether the data carries the mandatory OTP/MachineID pair.
func (d Data) Complete() bool {
	return d.OTP != "" && d.MachineID != ""
}

// WithDefaults fills in locale/timezone defaults where they are missing, so
// callers always have a usable set of values.
func (d Data) WithDefaults() Data {
	if d.Locale == "" {
		d.Locale = "en_US"
	}

	if d.Timezone == "" {
		d.Timezone = "PST"
	}

	return d
}

// Provider produces anisette data. Implementations must be safe for reuse.
type Provider interface {
	Fetch(ctx context.Context) (Data, error)
}

// ParseJSON parses the flat JSON object returned by SideStore-style public
// anisette servers, e.g. {"X-Apple-I-MD": "...", "X-Apple-I-MD-M": "..."}.
func ParseJSON(text []byte) Data {
	var raw map[string]interface{}
	if err := json.Unmarshal(text, &raw); err != nil {
		return Data{}
	}

	return Data{
		OTP:           str(raw["X-Apple-I-MD"]),
		MachineID:     str(raw["X-Apple-I-MD-M"]),
		LocalUserUUID: str(raw["X-Apple-I-MD-LU"]),
		DeviceID:      str(raw["X-Mme-Device-Id"]),
		ClientInfo:    str(raw["X-MMe-Client-Info"]),
		SerialNo:      str(raw["X-Apple-I-SRL-NO"]),
		RoutingInfo:   str(raw["X-Apple-I-MD-RINFO"]),
		ClientTime:    str(raw["X-Apple-I-Client-Time"]),
		Locale:        str(raw["X-Apple-Locale"]),
		Timezone:      str(raw["X-Apple-I-TimeZone"]),
	}
}

// ParseKeyValue parses "Key: Value" output from a local anisette binary, one
// header per line.
func ParseKeyValue(output string) Data {
	d := Data{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if colon := strings.Index(line, ":"); colon >= 0 {
			key := strings.TrimSpace(line[:colon])
			val := strings.TrimSpace(line[colon+1:])
			switch key {
			case "X-Apple-I-MD":
				d.OTP = val
			case "X-Apple-I-MD-M":
				d.MachineID = val
			case "X-Apple-I-MD-LU":
				d.LocalUserUUID = val
			case "X-Apple-I-MD-RINFO":
				d.RoutingInfo = val
			case "X-Apple-I-SRL-NO":
				d.SerialNo = val
			case "X-Apple-I-Client-Time":
				d.ClientTime = val
			case "X-Apple-Locale":
				d.Locale = val
			case "X-Apple-I-TimeZone":
				d.Timezone = val
			case "X-MMe-Client-Info":
				d.ClientInfo = val
			case "X-Mme-Device-Id":
				d.DeviceID = val
			}
		}
	}

	return d
}

func str(v interface{}) string {
	s, _ := v.(string)
	return s
}
