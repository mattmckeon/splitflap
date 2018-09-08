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
		{"11:40AM", "Haverhill Line", "4", "Now boarding"},
		{"12:20PM", "Newburyport/Rockport Line", "", "On time"},
		{"1:10PM", "Fitchburg Line", "",  "On time"},
		{"1:30PM", "Newburyport/Rockport Line", "", "On time"},
	}
	assert.Equal(t, expected, actual)
}
