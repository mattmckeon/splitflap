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
			Status string `json:"status"`
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
			// at each step of the path. There are several JSONpath
			// libraries but they seem to be focused on fetching single
			// values, and are a poor fit for this sort of thing where
			// we're trying to cross-reference relationships.
			// I tried a couple of different approaches and this seemed the
			// least bad.
			Headsign string `json:"headsign"`
			PlatformCode string `json:"platform_code"`
			RouteType int `json:"type"`
		} `json:"attributes"`
		Type string `json:"type"`
		Id string `json:"id"`
	} `json:"included"`
}

type Params struct {
	MinTime string `url:"filter[min_time],omitempty"`
	MaxTime string `url:"filter[max_time],omitempty"`
	Stop    string `url:"filter[stop],omitempty"`
	Include string `url:"include,omitempty"`
	Sort string `url:"sort,omitempty"`
}

type Departure struct {
	TimeLabel string
	Destination string
	Track string
	Status string
}

type MbtaService struct {
	sling *sling.Sling
	client *http.Client
}

func NewMbtaService() *MbtaService {
	httpClient := &http.Client{
		Timeout: time.Second * 10,
	}
	return &MbtaService{
		sling: sling.New().Client(httpClient).Base(baseURL),
		client: httpClient,
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

	departures := []Departure{}
	for _, result := range apiResponse.Data {
		// Don't show trains for which we don't have a prediction.
		if (result.Attributes.DepartureTime != "") {
			// Our route index only includes commuter rail trains;
			// we can skip anything that isn't in the index (e.g. green line etc)
			if _, ok := routeIndex[result.Relationships.Route.Data.Id]; ok {
				d := Departure{}
				d.Destination = destinationIndex[result.Relationships.Trip.Data.Id]
				// TODO: handle time parse error
				t, _ := time.Parse(time.RFC3339, result.Attributes.DepartureTime)
				d.TimeLabel = t.Format("3:04PM")
				d.Status = result.Attributes.Status
				d.Track = trackIndex[result.Relationships.Stop.Data.Id]
				departures = append(departures, d)
			}
		}
	}
	return departures, nil
}

func (s *MbtaService) ListDepartures(params *Params) ([]Departure, error) {
	sling := s.sling.New().Path("predictions").QueryStruct(params)
	// TODO: handle request error
	req, err := sling.Request()
	fmt.Printf("request: %v", req)
	apiResponse := new(ApiV3Response)
	// TODO: handle response error
	_, err = sling.ReceiveSuccess(apiResponse)
	departures, err := ExtractDepartures(apiResponse)
	return departures, err
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
			Sort: "departure_time",
		}
		client := NewMbtaService()
		// TODO: handle request errors
		northStation, _ := client.ListDepartures(params)
		params.Stop = "place-sstat"
		southStation, _ := client.ListDepartures(params)
		c.HTML(http.StatusOK, "index.tmpl.html", gin.H{
			"northStation": northStation,
			"southStation": southStation,
		})
	})

	router.Run(":" + port)
}
