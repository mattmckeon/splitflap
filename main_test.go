package main

import (
	"encoding/json"
	"os"
	"io/ioutil"
	"testing"
	"github.com/stretchr/testify/assert"
)

func readPredictions() (*ApiV3Response, error) {
	f, err := os.Open("testdata/predictions.json")
	if (err != nil) {
	}
	defer f.Close()

	byteValue, err := ioutil.ReadAll(f)
	var apiResponse = new(ApiV3Response)
	err = json.Unmarshal(byteValue, apiResponse)
	return apiResponse, err
}

func TestParse(t *testing.T) {
	apiResponse, _ := readPredictions()
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
