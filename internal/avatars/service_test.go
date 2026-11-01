package avatars

import "testing"

func TestAvatarPathStaysInsideStorage(t *testing.T) {
	service, err := NewService(nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.path("user-id"); err != nil {
		t.Fatalf("safe path rejected: %v", err)
	}
	for _, value := range []string{"", ".", "..", "../user", "folder/user"} {
		if _, err := service.path(value); err == nil {
			t.Fatalf("unsafe path %q accepted", value)
		}
	}
}

func TestAllowedAvatarMediaTypes(t *testing.T) {
	for _, value := range []string{"image/jpeg", "image/png", "image/gif", "image/webp"} {
		if !allowedMediaType(value) {
			t.Fatalf("expected %s to be allowed", value)
		}
	}
	for _, value := range []string{"image/svg+xml", "text/html", "application/octet-stream"} {
		if allowedMediaType(value) {
			t.Fatalf("expected %s to be rejected", value)
		}
	}
}
