package main

import (
	"encoding/json"
	"os"
	"io/ioutil"
	"net/http"
	"testing"
	"github.com/stretchr/testify/assert"
	"gopkg.in/h2non/gock.v1"
)

func readPredictions() (*ApiV3Response, error) {
	f, err := os.Open("testdata/predictions.json")
	if (err != nil) {
		return nil, err
	}
	defer f.Close()

	byteValue, err := ioutil.ReadAll(f)
	var apiResponse = new(ApiV3Response)
	err = json.Unmarshal(byteValue, apiResponse)
	return apiResponse, err
}


func TestParse(t *testing.T) {
	apiResponse, err := readPredictions()
	if err != nil {
		assert.FailNow(t, "Failed to open test fixture")
	}
	actual, _ := ExtractDepartures(apiResponse)

	expected :=  []Departure {
		{"11:50AM", "Readville", "10", "Now boarding"},
		{"12:40PM", "Worcester", "", "On time"},
		{"12:50PM", "Readville", "",  "On time"},
		{"1:05PM", "Providence", "", "On time"},
		{"1:20PM", "Forge Park/495", "", "On time"},
	}
	assert.Equal(t, expected, actual)
}

func TestRateLimitError(t *testing.T) {
	defer gock.Off()
	f, err := os.Open("testdata/error-429.json")
	if (err != nil) {
		assert.FailNow(t, "Failed to open test fixture")
	}

	gock.New(MbtaApiV3BaseUrl).
		Get("/predictions").
		Reply(429).
		Body(f)

	httpClient := &http.Client{}
	gock.InterceptClient(httpClient)

	departures, err := NewMbtaService(httpClient).ListDepartures(&Params{})
	assert.Nil(t, departures)
	assert.EqualError(t, err, "MBTA API error: You have exceeded your allowed usage rate.")
}
