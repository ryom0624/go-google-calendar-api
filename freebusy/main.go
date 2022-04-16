package main

import (
	"context"
	"encoding/json"
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

	const days = 14
	timeMax := time.Now().Add(24 * days * time.Hour).Format(time.RFC3339)
	timeMin := time.Now().Format(time.RFC3339)
	fmt.Println(timeMin)
	fmt.Println(timeMax)

	// sample calendar ids
	freeBusyItem := []*calendar.FreeBusyRequestItem{
		{Id: "kg090637fo0f1lg5s3ham2bhk8@group.calendar.google.com"},
		{Id: "0lqtb45e5rpi3jmvjs4kcrrh94@group.calendar.google.com"},
		{Id: "7j4hmerqr14ptp98p6b5p3io2k@group.calendar.google.com"},
	}

	freeBusyRequest := calendar.FreeBusyRequest{
		CalendarExpansionMax: 0,
		GroupExpansionMax:    0,
		Items:                freeBusyItem,
		TimeMax:              timeMax,
		TimeMin:              timeMin,
		TimeZone:             "Asia/Tokyo",
		ForceSendFields:      nil,
		NullFields:           nil,
	}

	freeBusyResponse, err := calendarService.Freebusy.Query(&freeBusyRequest).Do()
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(freeBusyResponse, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

	mapDate := make(map[string]uint64)
	for _, calendar := range freeBusyResponse.Calendars {
		for _, busy := range calendar.Busy {
			if busy == nil {

			}
			calcBit(mapDate, busy)
		}
	}
	for k, v := range mapDate {
		fmt.Printf("%v %064b\n", k, v)
	}
}

func calcBit(mapDate map[string]uint64, timePeriod *calendar.TimePeriod) error {
	startTime, err := time.Parse(time.RFC3339, timePeriod.Start)
	if err != nil {
		return err
	}
	endTime, err := time.Parse(time.RFC3339, timePeriod.End)
	if err != nil {
		return err
	}
	startTimeBit := uint64(startTime.Hour() * 2)
	if startTime.Minute() >= 30 {
		startTimeBit++
	}
	endTimeBit := uint64(endTime.Hour() * 2)
	if endTime.Minute() == 0 {
		endTimeBit--
	} else if endTime.Minute() > 30 {
		endTimeBit++
	}

	date := startTime.Format("2006/01/02")
	tmp := make(map[string]uint64)
	if _, ok := mapDate[date]; !ok {
		for i := startTimeBit; i <= endTimeBit; i++ {
			mapDate[date] |= 1 << i
		}
	} else {
		for i := startTimeBit; i <= endTimeBit; i++ {
			tmp[date] |= 1 << i
		}
		mapDate[date] = mapDate[date] & tmp[date]
	}

	return nil
}

/*
       24:00 23:00 22:00 21:00 20:00 19:00 18:00 17:00 16:00 15:00 14:00 13:00 12:00 11:00 10:00 09:00 08:00 07:00 06:00 05:00 04:00 03:00 02:00 01:00
time :  1  2  3  4  5  6  7  8  9 10 11 12 13 14 15 16 17 18 19 20 21 22 23 24 25 26 27 28 29 30 31 32 33 34 35 36 37 38 39 40 41 42 43 44 45 46 47 48

b_a  :  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  1  1  1  1  0  0  0  1  1  0  0  0  0  1  0  0  0  1  1  0  0  0  0  0  0  0
b_b  :  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  1  1  1  0  0  0  1  1  1  1  1  0  1  0  1  1  0  1  0  0  0  0  0  0  0  0  0  0
b_c  :  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  0  1  1  1  1  1  1  1  1  1  1  0  1  1  0  1  1  0  1  1  1  0  0  0  0  0  0  0
-------------------------------------------------------------------------------------------------------------------------------------------------------
aki  :  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  0  0  0  0  0  0  1  1  0  0  0  0  1  0  0  0  0  0  0  0  0  0  0  0  0

0 空き
1 空いてない
*/
