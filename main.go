package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/dghubble/sling"
	"github.com/gin-gonic/gin"
	"github.com/google/jsonapi"
)

const MbtaApiV3BaseUrl = "https://api-v3.mbta.com/"

// Prediction represents an MBTA API prediction and its relationships.
// We only define the fields we need to unmarshal from the JSONAPI response.
type Prediction struct {
	Id            string    `jsonapi:"primary,prediction"`
	DepartureTime string    `jsonapi:"attr,departure_time"`
	Status        string    `jsonapi:"attr,status"`
	Route         *Route    `jsonapi:"relation,route,omitempty"`
	Trip          *Trip     `jsonapi:"relation,trip,omitempty"`
	Stop          *Stop     `jsonapi:"relation,stop,omitempty"`
	Schedule      *Schedule `jsonapi:"relation,schedule,omitempty"`
}

// Route represents a route as defined in the MBTA API.
// We only define the fields we need to unmarshal from the JSONAPI response.
type Route struct {
	Id             string   `jsonapi:"primary,route"`
	Type           int      `jsonapi:"attr,type"`
	DirectionNames []string `jsonapi:"attr,direction_names"`
}

// Schedule represents a scheduled departure or arrival in the MBTA API.
// We only define the fields we need to unmarshal from the JSONAPI response.
type Schedule struct {
	Id            string `jsonapi:"primary,schedule"`
	DepartureTime string `jsonapi:"attr,departure_time"`
}

// Stop represents a stop or station as defined in the MBTA API.
// We only define the fields we need to unmarshal from the JSONAPI response.
type Stop struct {
	Id           string `jsonapi:"primary,stop"`
	PlatformCode string `jsonapi:"attr,platform_code"`
}

// Trip represents a journey as defined in the MBTA API.
// We only define the fields we need to unmarshal from the JSONAPI response.
type Trip struct {
	Id          string `jsonapi:"primary,trip"`
	Headsign    string `jsonapi:"attr,headsign"`
	DirectionId int    `jsonapi:"attr,direction_id"`
}

// ApiV3Error is the base type used to unmarshall an error from MBTA APIv3.
type ApiV3Error struct {
	Errors []struct {
		Status string `json:"status"`
		Source struct {
			Parameter string `json:"parameter"`
		} `json:"source"`
		Detail string `json:"detail"`
		Code   string `json:"code"`
	} `json:"errors"`
}

// Error implements the Golang error interface for ApiV3Error.
func (e ApiV3Error) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("MBTA API error: %v", e.Errors[0].Detail)
	} else {
		return fmt.Sprintf("MBTA API error: %+v", e.Errors)
	}
}

// ParseError is used to gather errors resulting from parsing the API response
// to generate the departure board rows.
type ParseError struct {
	Errors []error
}

// Error implements the Golang error interface for ParseError.
func (e ParseError) Error() string {
	return fmt.Sprintf("Parse error: %+v", e.Errors)
}

// Params defines the query parameters sent via the Sling library.
// The field tags map each value to a URL parameter.
type Params struct {
	Stop    string `url:"filter[stop],omitempty"`
	Include string `url:"include,omitempty"`
	Sort    string `url:"sort,omitempty"`
}

// Departure represents each row in our departure board.
type Departure struct {
	TimeLabel   string
	Destination string
	Track       string
	Status      string
}

// DepartureBoard encapsulates the title, rows, and any errors for each board.
type DepartureBoard struct {
	Title      string
	Departures []Departure
	Error      error
}

// MbtaService is a base interface for fetching and parsing departures.
type MbtaService interface {
	ListDepartures(place string) ([]Departure, error)
}

// MbtaServiceImpl wraps the Sling request handle and underlying http client.
type MbtaServiceImpl struct {
	sling  *sling.Sling
	client *http.Client
}

// NewMbtaServiceImpl creates and returns a new instance of MbtaServiceImpl
// (visible so we can pass mocks for testing).
func NewMbtaServiceImpl(httpClient *http.Client) *MbtaServiceImpl {
	return &MbtaServiceImpl{
		sling:  sling.New().Client(httpClient).Base(MbtaApiV3BaseUrl),
		client: httpClient,
	}
}

// NewHttpClient creates a new HTTP client configured with a timeout.
func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

// ListDepartures is an implementation of the MbtaService ListDepartures method
// that fetches commuter departure board information from the MBTA APIv3
// predictions endpoint.
func (s *MbtaServiceImpl) ListDepartures(place string) ([]Departure, error) {
	sling := s.sling.New().Path("predictions").QueryStruct(&Params{
		Stop:    place,
		Include: "route,stop,trip,schedule",
		Sort:    "departure_time",
	})

	// Dump the request to logs for debugging
	req, err := sling.Request()
	fmt.Printf("request: %v", req)

	// Unfortunately the Golang JSONAPI library is intended for services, so the
	// response parsing doesn't handle errors as gracefully as we'd like.
	// We need to check the status code and try to unmarshall any errors we find.
	resp, err := s.client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			var apiError = new(ApiV3Error)
			err = json.NewDecoder(resp.Body).Decode(apiError)
			if err == nil {
				err = apiError
			}
		} else {
			rawPredictions, err := jsonapi.UnmarshalManyPayload(
				resp.Body, reflect.TypeOf(new(Prediction)))
			if err == nil {
				return ExtractDepartures(AsPredictions(rawPredictions))
			}
		}
	}
	return nil, err
}

// MbtaServiceTest is a test version of MbtaService useful for testing with
// canonical, non-live test responses from the API.
type MbtaServiceTest struct {
	JsonFile string
}

// ListDepartures is an implementation of the MbtaService ListDepartures method
// that ignores the provided place and loads test data from this test service's
// JsonFile.
func (s *MbtaServiceTest) ListDepartures(place string) ([]Departure, error) {
	f, err := os.Open(s.JsonFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	byteValue, err := ioutil.ReadAll(f)
	var apiError = new(ApiV3Error)
	err = json.Unmarshal(byteValue, apiError)
	if err != nil {
		return nil, err
	}
	if len(apiError.Errors) > 0 {
		return nil, apiError
	}
	rawPredictions, err := jsonapi.UnmarshalManyPayload(
		bytes.NewReader(byteValue), reflect.TypeOf(new(Prediction)))
	if err == nil {
		return ExtractDepartures(AsPredictions(rawPredictions))
	}
	return nil, err
}

// AsPredictions casts the raw unmarshalled JSON payload to the correct type.
func AsPredictions(rawPredictions []interface{}) []*Prediction {
	predictions := make([]*Prediction, len(rawPredictions))
	for i := range rawPredictions {
		predictions[i] = rawPredictions[i].(*Prediction)
	}
	return predictions
}

// ExtractDepartures is a helper function that extracts fields from an
// unmarshalled JSONAPI payload and returns a slice of rows corresponding to
// upcoming commuter rail departures. It assumes that the payload is a slice of
// pointers to
func ExtractDepartures(predictions []*Prediction) ([]Departure, error) {
	departures := []Departure{}
	parseError := new(ParseError)
	for _, prediction := range predictions {
		// We only want trains that match the following:
		// ✔ Have a valid departure time
		// ✔ On a commuter rail route (route.type == 2)
		// ✔ Are on an outbound trip
		if prediction.DepartureTime != "" &&
			prediction.Route.Type == 2 &&
			prediction.Route.DirectionNames[prediction.Trip.DirectionId] == "Outbound" {
			d := Departure{}
			d.Destination = prediction.Trip.Headsign
			pt, pterr := time.Parse(time.RFC3339, prediction.DepartureTime)
			if pterr == nil {
				d.TimeLabel = pt.Format("3:04PM")
			} else {
				err := fmt.Errorf("(Parse Error) %s", prediction.DepartureTime)
				parseError.Errors = append(parseError.Errors, err)
				d.TimeLabel = err.Error()
			}
			d.Status = prediction.Status
			if d.Status == "" && pterr == nil && prediction.Schedule != nil {
				// It's possible this is a delayed train, and we should reflect that.
				st, sterr := time.Parse(time.RFC3339, prediction.Schedule.DepartureTime)
				if sterr == nil && pt.After(st) {
					d.Status = "Delayed"
				}
			}
			d.Track = prediction.Stop.PlatformCode
			if d.Track == "" {
				d.Track = "TBD"
			}
			departures = append(departures, d)
		}
	}
	if len(parseError.Errors) > 0 {
		return departures, parseError
	} else {
		return departures, nil
	}
}

// Render is a helper function that fetches departures from the given service
// and outputs the corresponding HTML to the gin Context.
func Render(c *gin.Context, client MbtaService) {
	northStation := &DepartureBoard{
		Title: "North Station Information",
	}
	southStation := &DepartureBoard{
		Title: "South Station Information",
	}
	northStation.Departures, northStation.Error =
		client.ListDepartures("place-north")
	southStation.Departures, southStation.Error =
		client.ListDepartures("place-sstat")
	c.HTML(http.StatusOK, "index.tmpl.html", gin.H{
		"northStation": northStation,
		"southStation": southStation,
	})
}

func main() {
	port := os.Getenv("PORT")

	if port == "" {
		log.Fatal("$PORT must be set")
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.LoadHTMLGlob("templates/*.tmpl.html")
	router.Static("/static", "static")

	// The main route
	router.GET("/", func(c *gin.Context) {
		Render(c, NewMbtaServiceImpl(NewHttpClient()))
	})

	// A test route that returns canned prediction data.
	// Useful for tweaking CSS changes.
	router.GET("/test", func(c *gin.Context) {
		Render(c, &MbtaServiceTest{"testdata/predictions-delayed.json"})
	})

	// A test route that returns an API error.
	// Useful for tweaking CSS changes.
	router.GET("/testerror", func(c *gin.Context) {
		Render(c, &MbtaServiceTest{"testdata/error-429.json"})
	})

	router.Run(":" + port)
}
