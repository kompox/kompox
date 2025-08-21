package naming

import "testing"

func TestNewHashesStabilityAndIndependence(t *testing.T) {
	h1 := NewHashes("svc", "prv", "cls1", "app")
	h2 := NewHashes("svc", "prv", "cls1", "app")
	if h1 != h2 {
		t.Fatalf("hashes not stable: %#v vs %#v", h1, h2)
	}
	if h1.AppID == h1.AppInstance {
		t.Fatalf("AppID and AppInstance must differ when cluster dimension included")
	}
	h3 := NewHashes("svc", "prv", "cls2", "app")
	if h1.AppID != h3.AppID {
		t.Fatalf("AppID should be cluster independent: %s vs %s", h1.AppID, h3.AppID)
	}
	if h1.AppInstance == h3.AppInstance {
		t.Fatalf("AppInstance should change when cluster changes: %s == %s", h1.AppInstance, h3.AppInstance)
	}
}

func TestVolumeHashLength(t *testing.T) {
	h := VolumeHash("some-volume-handle")
	if len(h) != 6 {
		t.Fatalf("expected volume hash length 6, got %d", len(h))
	}
}
