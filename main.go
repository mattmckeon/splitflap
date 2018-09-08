package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
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
	Type        int    `json:"type"`
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
	Time time.Time
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
	sling := s.sling.New().Path("schedules").QueryStruct(params)
	req, err := sling.Request()
	fmt.Printf("request: %v", req)
	resp, err := sling.ReceiveSuccess(departures)
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

	// TODO: use predictions API	
    // example: https://api-v3.mbta.com/predictions?filter%5Bmax_time%5D=09%3A13&filter%5Bmin_time%5D=08%3A43&filter%5Bstop%5D=place-north&include=route,stop
	// included stop has platform: {"attributes": {"platform_code":"6"}, id "North Station-06"}
	// Prediction: {"attributes": {"departure_time":"XYZ", "status":"All aboard"}}
	// Relationships: route, stop
	router.GET("/", func(c *gin.Context) {
		now := time.Now()
		params := &ScheduleParams{
			MinTime: now.Format("15:04"),
			MaxTime: now.Add(time.Minute * 30).Format("15:04"),
			Stop:    "place-north",
			Include: "route"}
		client := NewClient(NewHttpClient())
		// TODO: handle request error
		departureResponse, _, _ := client.MbtaService.ListDepartures(params)
		routeIndex := make(map[string]RouteProperties)
		for _, entry := range departureResponse.RouteIndex {
			routeIndex[entry.Id] = entry.Route
		}
		nextDepartures := []Departure{}
		for _, dr := range departureResponse.Departures {
			routeId := dr.Relationships.Route.Data.Id
			// TODO: make the type a constant
			if routeIndex[routeId].Type == 2 {
				d := Departure{}
				// TODO: handle time parse error
				d.Time, _ = time.Parse(time.RFC3339, dr.Attributes.DepartureTime)
				d.TimeLabel = d.Time.Format("3:04PM")
				d.Route = routeIndex[routeId]
				nextDepartures = append(nextDepartures, d)
			}
		}
		sort.Slice(nextDepartures, func(i, j int) bool {
			return nextDepartures[i].Time.Before(nextDepartures[j].Time)
		})
		
		c.HTML(http.StatusOK, "index.tmpl.html", gin.H{
			"departures": nextDepartures,
		})
	})

	router.Run(":" + port)
}
