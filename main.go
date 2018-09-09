package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dghubble/sling"
	"github.com/gin-gonic/gin"
)

const baseURL = "https://api-v3.mbta.com/"

type ApiV3Response struct {
	Data []struct {
		Attributes struct {
			DepartureTime string `json:"departure_time"`
			Status        string `json:"status"`
		} `json:"attributes"`
		Relationships struct {
			Route struct {
				Data struct {
					Id string `json:"id"`
				} `json:"data"`
			} `json:"route"`
			Stop struct {
				Data struct {
					Id string `json:"id"`
				} `json:"data"`
			} `json:"stop"`
			Trip struct {
				Data struct {
					Id string `json:"id"`
				} `json:"data"`
			} `json:"trip"`
		} `json:"relationships"`
		Id string `json:"id"`
	} `json:"data"`
	Included []struct {
		Attributes struct {
			// This is pretty hacky - we're basically merging the three
			// different types we could be getting back in the included list
			// (routes, trips and stops). Go has great support for parsing
			// of well-formed and strongly typed JSON object graphs,
			// and OK support for generic handling of simple and shallow
			// JSON responses. Things get hairy though when we have deeply
			// nested polymorphic trees like this one - you need to cast
			// at each step of the path during lookup. There are several
			// JSONpath libraries but they seem to be focused on fetching single
			// values, and are a poor fit for this sort of thing where
			// we're trying to cross-reference relationships.
			// I tried a couple of different approaches and this seemed the
			// least bad.
			Headsign     string `json:"headsign"`
			PlatformCode string `json:"platform_code"`
			RouteType    int    `json:"type"`
		} `json:"attributes"`
		Type string `json:"type"`
		Id   string `json:"id"`
	} `json:"included"`
}

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

func (e ApiV3Error) Error() string {
	return fmt.Sprintf("MBTA APIv3: %+v", e.Errors)
}

type ParseError struct {
	Errors []error
}

func (e ParseError) Error() string {
	return fmt.Sprintf("Parse error: %+v", e.Errors)
}

type Params struct {
	MinTime string `url:"filter[min_time],omitempty"`
	MaxTime string `url:"filter[max_time],omitempty"`
	Stop    string `url:"filter[stop],omitempty"`
	Include string `url:"include,omitempty"`
	Sort    string `url:"sort,omitempty"`
}

type Departure struct {
	TimeLabel   string
	Destination string
	Track       string
	Status      string
}

type DepartureBoard struct {
	Title string
	Departures []Departure
	Error error
}

type MbtaService struct {
	sling  *sling.Sling
	client *http.Client
}

func NewMbtaService(httpClient *http.Client) *MbtaService {
	return &MbtaService{
		sling:  sling.New().Client(httpClient).Base(baseURL),
		client: httpClient,
	}
}

func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
	}
}

func ExtractDepartures(apiResponse *ApiV3Response) ([]Departure, error) {
	trackIndex := make(map[string]string)
	routeIndex := make(map[string]bool)
	destinationIndex := make(map[string]string)
	for _, entry := range apiResponse.Included {
		// Only index commuter rail routes so we can filter predictions.
		if entry.Type == "route" && entry.Attributes.RouteType == 2 {
			routeIndex[entry.Id] = true
		}
		if entry.Type == "stop" {
			trackIndex[entry.Id] = entry.Attributes.PlatformCode
		}
		if entry.Type == "trip" {
			destinationIndex[entry.Id] = entry.Attributes.Headsign
		}
	}

	parseError := new(ParseError)
	departures := []Departure{}
	for _, result := range apiResponse.Data {
		// Don't show trains for which we don't have a prediction or status
		if result.Attributes.DepartureTime != "" && result.Attributes.Status != "" {
			// Our route index only includes commuter rail trains;
			// we can skip anything that isn't in the index (e.g. green line etc)
			if _, ok := routeIndex[result.Relationships.Route.Data.Id]; ok {
				d := Departure{}
				d.Destination = destinationIndex[result.Relationships.Trip.Data.Id]
				t, err := time.Parse(time.RFC3339, result.Attributes.DepartureTime)
				if err == nil {
					d.TimeLabel = t.Format("3:04PM")
				} else {
					err := fmt.Errorf("(Parse Error) %s", result.Attributes.DepartureTime)
					parseError.Errors = append(parseError.Errors, err)
					d.TimeLabel = err.Error()
				}
				d.Status = result.Attributes.Status
				d.Track = trackIndex[result.Relationships.Stop.Data.Id]
				departures = append(departures, d)
			}
		}
	}
	if len(parseError.Errors) > 0 {
		return departures, parseError
	} else {
		return departures, nil
	}
}

func (s *MbtaService) ListDepartures(params *Params) ([]Departure, error) {
	sling := s.sling.New().Path("predictions").QueryStruct(params)
	// Dump the request to logs for debugging
	req, err := sling.Request()
	fmt.Printf("request: %v", req)

	apiResponse := new(ApiV3Response)
	apiError := new(ApiV3Error)
	_, err = sling.Receive(apiResponse, apiError)
	if err == nil {
		err = apiError
	}
	if len(apiError.Errors) == 0 {
		return ExtractDepartures(apiResponse)
	} else {
		return nil, err
	}
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

	router.GET("/", func(c *gin.Context) {
		params := &Params{
			Stop:    "place-north",
			Include: "route,stop,trip",
			Sort:    "departure_time",
		}
		client := NewMbtaService(NewHttpClient())
		northStation := &DepartureBoard{
			Title: "North Station",
		}
		southStation := &DepartureBoard{
			Title: "South Station",
		}
		northStation.Departures, northStation.Error = client.ListDepartures(params)
		params.Stop = "place-sstat"
		southStation.Departures, southStation.Error = client.ListDepartures(params)
		c.HTML(http.StatusOK, "index.tmpl.html", gin.H{
			"northStation": northStation,
			"southStation": southStation,
		})
	})

	router.Run(":" + port)
}
