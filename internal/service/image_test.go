package service

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImageHandlerRequiresCallerWindow(t *testing.T) {
	app, _, _ := mutationTestApp(t)
	handler := ImageHandler(app)

	get := httptest.NewRequest("GET", "http://kbimg.localhost/raws/x.png?vault=v", nil)
	getRec := httptest.NewRecorder()
	handler(getRec, get)
	if getRec.Code != 404 {
		t.Errorf("GET without caller = %d", getRec.Code)
	}

	post := httptest.NewRequest("POST", "http://kbimg.localhost/__upload?vault=v", strings.NewReader("x"))
	postRec := httptest.NewRecorder()
	handler(postRec, post)
	if postRec.Code != 404 {
		t.Errorf("POST without caller = %d", postRec.Code)
	}
}
