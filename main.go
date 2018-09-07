package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dghubble/sling"
	"github.com/gin-gonic/gin"
)

const baseURL = "https://api-v3.mbta.com/"

type ScheduleParams struct {
	MinTime string `url:"filter[min_time],omitempty"`
	MaxTime string `url:"filter[max_time],omitempty"`
	Stop    string `url:"filter[stop],omitempty"`
	Include string `url:"include,omitempty"`
}

type RouteProperties struct {
	Color       string `json:"color"`
	TextColor   string `json:"text_color"`
	Description string `json:"description"`
	LongName    string `json:"long_name"`
}

type DepartureResponse struct {
	Departures []struct {
		Attributes struct {
			DepartureTime string `json:"departure_time"`
		} `json:"attributes"`
		Relationships struct {
			Route struct {
				Data struct {
					Id string `json:"id"`
				} `json:"data"`
			} `json:"route"`
		} `json:"relationships"`
	} `json:"data"`
	RouteIndex []struct {
		Route RouteProperties `json:"attributes"`
		Id string `json:"id"`
	} `json:"included"`
}

type Departure struct {
	TimeLabel string
	Route RouteProperties
}

type MbtaService struct {
	sling *sling.Sling
}

func NewMbtaService(httpClient *http.Client) *MbtaService {
	return &MbtaService{
		sling: sling.New().Client(httpClient).Base(baseURL),
	}
}

func (s *MbtaService) ListDepartures(params *ScheduleParams) (DepartureResponse, *http.Response, error) {
	departures := new(DepartureResponse)
	resp, err := s.sling.New().Path("schedules").QueryStruct(params).ReceiveSuccess(departures)
	return *departures, resp, err
}

type Client struct {
	MbtaService *MbtaService
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{
		MbtaService: NewMbtaService(httpClient),
	}
}

func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Second * 10,
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
		now := time.Now()
		params := &ScheduleParams{
			MinTime: now.Format("14:00"),
			MaxTime: now.Add(time.Minute * 30).Format("14:00"),
			Stop:    "place-north",
			Include: "route"}
		client := NewClient(NewHttpClient())
		departureResponse, _, _ := client.MbtaService.ListDepartures(params)
		routeIndex := make(map[string]RouteProperties)
		for _, entry := range departureResponse.RouteIndex {
			routeIndex[entry.Id] = entry.Route
		}
		departures := []Departure{}
		for _, d := range departureResponse.Departures {
			t, _ := time.Parse(time.RFC3339, d.Attributes.DepartureTime)
			departures = append(departures, Departure{
				TimeLabel: t.Format("12:00PM"),
				Route: routeIndex[d.Relationships.Route.Data.Id],
			})
		}
		c.HTML(http.StatusOK, "index.tmpl.html", gin.H{
			"departures": departures,
		})
	})

	router.Run(":" + port)
}
