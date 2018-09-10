package main

import (
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func TestParse(t *testing.T) {
	actual, _ := (&MbtaServiceTest{"testdata/predictions.json"}).ListDepartures("")

	expected := []Departure{
		{"11:50AM", "Readville", "10", "Now boarding"},
		{"12:40PM", "Worcester", "TBD", "On time"},
		{"12:50PM", "Readville", "TBD", "On time"},
		{"1:05PM", "Providence", "TBD", "On time"},
		{"1:20PM", "Forge Park/495", "TBD", "On time"},
	}
	assert.Equal(t, expected, actual)
}

func TestRateLimitError(t *testing.T) {
	defer gock.Off()
	f, err := os.Open("testdata/error-429.json")
	if err != nil {
		assert.FailNow(t, "Failed to open test fixture")
	}

	gock.New(MbtaApiV3BaseUrl).
		Get("/predictions").
		Reply(429).
		Body(f)

	httpClient := &http.Client{}
	gock.InterceptClient(httpClient)

	departures, err := NewMbtaServiceImpl(httpClient).ListDepartures("")
	assert.Nil(t, departures)
	assert.EqualError(t, err, "MBTA API error: You have exceeded your allowed usage rate.")
}
