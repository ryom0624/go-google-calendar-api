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

const defaultTimeZone = "Asia/Tokyo"

// CalendarBits 日付ごとに予定の状況を管理する
//
// ex:
// CalendarBits: {
// 	"2022/04/16" : {
// 		"example@gmail.com" : 000000000000000000001111110001100001000110000000
// 		"example2@gmail.com": 000000000000000000001111110001100001000110000000
// 	},
// 	"2022/04/17" : {
// 		"example@gmail.com" : 000000000000000000001111110001100001000110000000
// 		"example2@gmail.com": 000000000000000000001111110001100001000110000000
// 	},
// }
var CalendarBits = make(map[string]map[string]uint64)

// BitsToCalendars レスポンス用
//
// ex:
// BitsToCalendar: [
// 	{
// 		date : {
// 			value: "2022/04/16"
// 			text: "04/16"
// 			weekday: "土"
// 		},
// 		times: [
// 			{
// 				value: "2022-04-16T19:00+09:00"
// 				text: "19:00"
// 			},
// 			{
// 				value: "2022-04-16T19:30+09:00"
// 				text: "19:30"
// 			},
// 			{
// 				value: "2022-04-16T19:00+09:00"
// 				text: "20:00"
// 			},
// 		],
//
// 	},
// 	{
// 		date : {
// 			value: "2022/04/17"
// 			text: "04/17"
// 			weekday: "土"
// 		},
// 		times: [
// 			{
// 				value: "2022-04-17T19:00+09:00"
// 				text: "19:00"
// 			},
// 			{
// 				value: "2022-04-17T19:30+09:00"
// 				text: "19:30"
// 			},
// 			{
// 				value: "2022-04-17T19:00+09:00"
// 				text: "20:00"
// 			}
// 		],
// 	}
// }
type BitsToCalendars []BitsToCalendar

type BitsToCalendar struct {
	BitsToCalendarDates []BitsToCalendarDate `json:"dates"`
	BitsToCalendarTimes []BitsToCalendarTime `json:"times"`
}

type BitsToCalendarDate struct {
	value   string
	text    string
	weekday string
}
type BitsToCalendarTime struct {
	value string
	text  string
}

// Event calendarEventsからアプリ用に変換したもの
type Event struct {
	CalendarId    string
	CalendarName  string
	Title         string
	IsAllDate     bool
	StartDateTime time.Time
	EndDateTime   time.Time
}

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

	const days = 2
	startMinTime := 8
	endMaxTime := 20
	businessTimeRange := endMaxTime - startMinTime
	fmt.Println(businessTimeRange)

	datetimeMax := time.Now().AddDate(0, 0, days)
	datetimeMin := time.Now().AddDate(0, 0, 1)
	timeMax := datetimeMax.Format(time.RFC3339)
	timeMin := datetimeMin.Format(time.RFC3339)
	fmt.Println(timeMin)
	fmt.Println(timeMax)

	// sample calendar ids
	calendarIds := []string{
		"kg090637fo0f1lg5s3ham2bhk8@group.calendar.google.com",
		"0lqtb45e5rpi3jmvjs4kcrrh94@group.calendar.google.com",
		"7j4hmerqr14ptp98p6b5p3io2k@group.calendar.google.com",
	}

	for _, id := range calendarIds {
		events, err := calendarService.Events.List(id).MaxResults(int64(250)).TimeMin(timeMin).TimeMax(timeMax).TimeZone(defaultTimeZone).Do()
		if err != nil {
			log.Fatal(err)
		}
		for _, item := range events.Items {
			event, err := NewEvent(id, events.Summary, item.Summary, item)
			if err != nil {
				log.Fatal(err)
			}
			// log.Printf("calendar name: %v\ttitle: %+v\tstart: %+v\tend: %+v\n", event.CalendarName, event.Title, event.StartDateTime, event.EndDateTime)

			// "2022/04/16": 000000000000000000001111110001100001000110000000
			mapDate := make(map[string]uint64)
			calcBit(mapDate, event)
			for date, v := range mapDate {
				if _, ok := CalendarBits[date]; !ok {
					CalendarBits[date] = map[string]uint64{id: v}
					continue
				}
				if _, ok := CalendarBits[date][id]; !ok {
					CalendarBits[date][id] = v
					continue
				}
				CalendarBits[date][id] |= v
			}
		}
	}
	// b, err := json.MarshalIndent(CalendarBits, "", "    ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(string(b))

	for _, id := range calendarIds {
		diffHours := datetimeMax.Sub(datetimeMin).Hours()
		tmpTime := datetimeMin
		for i := 0; i <= int(diffHours/24); i++ {
			if _, ok := CalendarBits[tmpTime.Format("2006/01/02")]; !ok {
				CalendarBits[tmpTime.Format("2006/01/02")] = map[string]uint64{id: 0 << 31}
				tmpTime = tmpTime.AddDate(0, 0, 1)
				continue
			}
			if _, ok := CalendarBits[tmpTime.Format("2006/01/02")][id]; !ok {
				CalendarBits[tmpTime.Format("2006/01/02")][id] = 0 << 31
			}
			tmpTime = tmpTime.AddDate(0, 0, 1)
		}
	}

	for date, v := range CalendarBits {
		fmt.Printf("------ %v ------\n", date)
		for email, bits := range v {
			fmt.Printf("\t%v %064b\n", email, bits)
		}
	}

	dateBits := make(map[string]uint64)
	for date, v := range CalendarBits {
		for _, bits := range v {
			if _, ok := dateBits[date]; !ok {
				dateBits[date] |= bits
			} else {
				dateBits[date] &= bits
			}
			// fmt.Printf("\t%064b\n", bits)
			// fmt.Printf("%v: %v %064b\n", date, email, bits)
		}
	}
	for date, v := range dateBits {
		fmt.Printf("///////// %v /////////\n", date)
		fmt.Printf("%064b\n", v)
	}

	for i, v := range dateBits {
		t, _ := time.Parse("2006/01/02", i)
		fmt.Printf("-------%v-------\n", i)
		for i := uint(startMinTime * 2); i < uint(endMaxTime*2); i++ {
			// fmt.Println(v & (1 << i))
			if v&(1<<i) != 1<<i {
				hour := i / 2
				minute := i % 2 * 30
				a := time.Date(t.Year(), t.Month(), t.Day(), int(hour), int(minute), 0, 0, time.Local)
				fmt.Println(a)
			}
		}
		fmt.Printf("-------%v-------\n", i)
	}

}

func timeParseRangeRFC3339(s, e string) (start, end time.Time, err error) {
	start, err = time.Parse(time.RFC3339, s)
	if err != nil {
		return start, end, err
	}
	end, err = time.Parse(time.RFC3339, e)
	if err != nil {
		return start, end, err
	}
	return start, end, err
}

func NewEvent(id, name, title string, item *calendar.Event) (*Event, error) {
	var isAllDate bool
	sTime, eTime, err := timeParseRangeRFC3339(item.Start.DateTime, item.End.DateTime)
	if err != nil {
		isAllDate = true
		// 終日イベントはitem.Start.Dateに値が入る
		sTime, eTime, err = timeParseRangeRFC3339(item.Start.Date+"T00:00:00+09:00", item.End.Date+"T23:59:59+09:00")
		if err != nil {
			return nil, err
		}
	}
	return &Event{CalendarId: id, CalendarName: name, Title: title, IsAllDate: isAllDate, StartDateTime: sTime, EndDateTime: eTime}, nil
}

func calcBit(mapDate map[string]uint64, event *Event) error {
	if event.IsAllDate {
		diffHours := event.EndDateTime.Sub(event.StartDateTime).Hours()
		mapKeyDate := event.StartDateTime
		for i := 0; i <= int(diffHours/24); i++ {
			mapDate[mapKeyDate.Format("2006/01/02")] = 0b11111111111111111111111111111111
			mapKeyDate = mapKeyDate.AddDate(0, 0, 1)
		}
		return nil
	}

	startTimeBit := uint64(event.StartDateTime.Hour() * 2)
	if event.StartDateTime.Minute() >= 30 {
		startTimeBit++
	}
	endTimeBit := uint64(event.EndDateTime.Hour() * 2)
	if event.EndDateTime.Minute() == 0 {
		endTimeBit--
	} else if event.EndDateTime.Minute() > 30 {
		endTimeBit++
	}

	date := event.StartDateTime.Format("2006/01/02")
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

b_a  :  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  1  0  0  0  0  1  1  1  1  1  1  0  0  0  1  1  0  0  0  0  1  0  0  0  0  0  0  0  0  0  0  0  0
b_b  :  0  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  0  0  1  1  1  1  1  0  0  0  1  1  1  1  1  0  1  0  1  1  0  0  0  0  0  0  0  0  0  0  0  0
b_c  :  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  1  0  0  1  1  0  1  1  1  1  1  1  1  1  1  1  0  1  1  0  1  0  0  0  0  0  0  0  0  0  0  0  0
b_d  :  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1  1
-------------------------------------------------------------------------------------------------------------------------------------------------------
aki  :  0  0  0  0  0  0  0  0  0  0  0  0  0  1  1  1  0  0  0  0  0  1  1  0  0  0  0  0  0  1  1  0  0  0  0  1  0  0  0  0  0  0  0  0  0  0  0  0

0 空き
1 空いてない
*/
