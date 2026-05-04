package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdminImageUploadHandler_InvalidMethod(t *testing.T) {
	app := &Application{}

	req := httptest.NewRequest("GET", "/admin/images/upload", nil)
	rr := httptest.NewRecorder()

	app.adminImageUploadHandler(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestAdminImageDeleteHandler_InvalidMethod(t *testing.T) {
	app := &Application{}

	req := httptest.NewRequest("PUT", "/admin/images/delete", nil)
	rr := httptest.NewRecorder()

	app.adminImageDeleteHandler(rr, req)

	assert.Equal(t, http.StatusMethodNotAllowed, rr.Code)
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Saint Nicholas", "saint-nicholas"},
		{"St. Seraphim", "st-seraphim"},
		{"Holy Trinity Church", "holy-trinity-church"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, slugify(tt.input))
	}
}

// More complex tests would require mocking S3 and ImageMagick
// which might be out of scope for a quick verification but 
// good to have in a real project.
