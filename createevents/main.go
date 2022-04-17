package main

import (
	"context"
	"fmt"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
	"io/ioutil"
	"log"
	"time"
)

func main() {
	ctx := context.Background()
	srvAcc, err := ioutil.ReadFile("./credentials/service_account.json")
	if err != nil {
		log.Fatal(err)
	}
	c, err := google.CredentialsFromJSON(ctx, srvAcc, calendar.CalendarScope)
	if err != nil {
		log.Fatal(err)
	}
	opt := option.WithCredentials(c)
	calendarService, err := calendar.NewService(ctx, opt)
	if err != nil {
		log.Fatal(err)
	}

	calendarId := "0lqtb45e5rpi3jmvjs4kcrrh94@group.calendar.google.com"
	// calendarId := "ryo.m0624@gmail.com"
	now := time.Now()
	start := time.Date(now.Year(), now.Month(), now.Day()+4, 10, 0, 0, 0, time.Local)
	end := time.Date(now.Year(), now.Month(), now.Day()+4, 15, 0, 0, 0, time.Local)

	event := calendar.Event{
		ConferenceData: &calendar.ConferenceData{
			CreateRequest: &calendar.CreateConferenceRequest{
				ConferenceSolutionKey: &calendar.ConferenceSolutionKey{
					Type: "hangoutsMeet",
				},
				RequestId: "7qxalsvy0e",
			},
		},
		Description: "API予定登録",
		End: &calendar.EventDateTime{
			DateTime: end.Format(time.RFC3339),
			TimeZone: "Asia/Tokyo",
		},
		Id: fmt.Sprintf("%v", now.Unix()),
		Start: &calendar.EventDateTime{
			DateTime: start.Format(time.RFC3339),
			TimeZone: "Asia/Tokyo",
		},
		Status:       "tentative",
		Summary:      fmt.Sprintf("APIでのテスト登録 %v", now.Unix()),
		Transparency: "opaque",
		// Transparency: "transparent",
		// Attendees: []*calendar.EventAttendee{}
	}

	e, err := calendarService.Events.Insert(calendarId, &event).Do()
	if err != nil {
		log.Printf("%v", err)
	}
	log.Printf("%+v", e)

}
