package seed

import "testing"

func TestAuditGapConfigJSONCoordMismatch(t *testing.T) {
	if _, err := ConfigJSON(Answers{Latitude: 1, Longitude: 2, Location: ""}); err == nil {
		t.Errorf("AUDIT: coords without location accepted")
	} else {
		t.Logf("coords-no-location rejected: %v", err)
	}
	if _, err := ConfigJSON(Answers{Latitude: 0, Longitude: 0, Location: "Seoul"}); err == nil {
		t.Errorf("AUDIT: location without coords accepted")
	} else {
		t.Logf("location-no-coords rejected: %v", err)
	}
}
