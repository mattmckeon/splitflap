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
		{"4:30PM", "Newburyport", "", "On time"},
		{"5:30PM", "Rockport", "", "On time"},
		{"5:35PM", "Haverhill", "",  "On time"},
		{"5:45PM", "Wachusett", "", "On time"},
	}
	assert.Equal(t, expected, actual)
}
