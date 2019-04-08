package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckNumberOfQueryStrings(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", "?target="+ts.URL+"&bla=foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusBadRequest)
	}
}

func TestCheckTargetQueryParameter(t *testing.T) {
	req, err := http.NewRequest("GET", "?bla=foo", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusBadRequest)
	}
}

func TestTargetIsUnavailable(t *testing.T) {
	req, err := http.NewRequest("GET", "?target=foo.com/bla", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadGateway {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusBadGateway)
	}
}

func TestResponseCodes(t *testing.T) {
	tests := []struct {
		StatusCode    int
		ShouldSucceed bool
	}{
		{200, true},
		{201, false},
		{299, false},
		{300, false},
		{403, false},
		{404, false},
		{504, false},
		{502, false},
		{503, false},
		{504, false},
	}

	for i, test := range tests {

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(test.StatusCode)
		}))
		defer ts.Close()

		req, err := http.NewRequest("GET", "?target="+ts.URL, nil)
		if err != nil {
			t.Fatal(err)
		}

		rr := httptest.NewRecorder()
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			probeHandler(w, r)
		})

		handler.ServeHTTP(rr, req)

		if status := rr.Code; status != http.StatusOK {
			if test.ShouldSucceed {
				t.Errorf("test probe #%v request handler returned wrong status code: %v, want %v", i, status, http.StatusOK)
			}
		}
	}
}

func TestDefaultTimeoutUsed(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(9 * time.Second)
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", "?target="+ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadGateway {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusBadGateway)
	}
}

func TestCustomTimeoutUsed(t *testing.T) {
	timeoutSeconds = 1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", "?target="+ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadGateway {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusBadGateway)
	}
}

func TestTimeoutFromHeaderUsed(t *testing.T) {
	timeoutSeconds = 1
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
	}))
	defer ts.Close()

	req, err := http.NewRequest("GET", "?target="+ts.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", "3")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		probeHandler(w, r)
	})

	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("probe request handler returned wrong status code: %v, want %v", status, http.StatusOK)
	}
}
