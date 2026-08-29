package anisette

import "testing"

func TestParseJSON(t *testing.T) {
	data := ParseJSON([]byte(`{
		"X-Apple-I-MD": "otp-value",
		"X-Apple-I-MD-M": "machine-id",
		"X-Apple-I-MD-LU": "local-uuid",
		"X-Apple-I-MD-RINFO": "17106176",
		"X-Apple-I-SRL-NO": "C02LKHBBFD57",
		"X-Apple-I-Client-Time": "2026-01-01T00:00:00Z",
		"X-Apple-Locale": "en_US",
		"X-Apple-I-TimeZone": "PST",
		"X-MMe-Client-Info": "<MacBookPro15,1>",
		"X-Mme-Device-Id": "device-id"
	}`))

	if data.OTP != "otp-value" {
		t.Fatalf("OTP = %q", data.OTP)
	}

	if data.MachineID != "machine-id" {
		t.Fatalf("MachineID = %q", data.MachineID)
	}

	if data.LocalUserUUID != "local-uuid" {
		t.Fatalf("LocalUserUUID = %q", data.LocalUserUUID)
	}

	if data.RoutingInfo != "17106176" {
		t.Fatalf("RoutingInfo = %q", data.RoutingInfo)
	}

	if data.SerialNo != "C02LKHBBFD57" {
		t.Fatalf("SerialNo = %q", data.SerialNo)
	}

	if data.ClientTime != "2026-01-01T00:00:00Z" {
		t.Fatalf("ClientTime = %q", data.ClientTime)
	}

	if data.Locale != "en_US" {
		t.Fatalf("Locale = %q", data.Locale)
	}

	if data.Timezone != "PST" {
		t.Fatalf("Timezone = %q", data.Timezone)
	}

	if data.ClientInfo != "<MacBookPro15,1>" {
		t.Fatalf("ClientInfo = %q", data.ClientInfo)
	}

	if data.DeviceID != "device-id" {
		t.Fatalf("DeviceID = %q", data.DeviceID)
	}
}

func TestParseKeyValue(t *testing.T) {
	data := ParseKeyValue("X-Apple-I-MD: otp-value\r\nX-Apple-I-MD-M: machine-id\r\nX-Apple-I-MD-RINFO: 17106176\r\n")

	if data.OTP != "otp-value" {
		t.Fatalf("OTP = %q", data.OTP)
	}

	if data.MachineID != "machine-id" {
		t.Fatalf("MachineID = %q", data.MachineID)
	}

	if data.RoutingInfo != "17106176" {
		t.Fatalf("RoutingInfo = %q", data.RoutingInfo)
	}
}

func TestDataComplete(t *testing.T) {
	if (Data{}).Complete() {
		t.Fatal("empty data should not be complete")
	}

	if !(Data{OTP: "otp", MachineID: "machine"}).Complete() {
		t.Fatal("data with OTP and MachineID should be complete")
	}

	if (Data{OTP: "otp"}).Complete() {
		t.Fatal("data without MachineID should not be complete")
	}
}

func TestDataWithDefaults(t *testing.T) {
	data := (Data{OTP: "otp", MachineID: "machine"}).WithDefaults()

	if data.Locale != "en_US" {
		t.Fatalf("Locale = %q, want en_US", data.Locale)
	}

	if data.Timezone != "PST" {
		t.Fatalf("Timezone = %q, want PST", data.Timezone)
	}

	custom := (Data{OTP: "otp", MachineID: "machine", Locale: "fr_FR", Timezone: "CET"}).WithDefaults()
	if custom.Locale != "fr_FR" || custom.Timezone != "CET" {
		t.Fatalf("WithDefaults overwrote explicit values: %+v", custom)
	}
}
