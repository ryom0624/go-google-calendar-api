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

const (
	DefaultTimeZone       = "Asia/Tokyo"
	FormatDate            = "2006/01/02"
	DaysRange             = 14
	EventTimeFrameMinutes = 30 // 1個の時間枠
	StartMinTimeHour      = 8
	EndMaxTimeHour        = 20
)

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

// FreeTimeSchedules レスポンス用
//
// ex:
// FreeTimeSchedule: [
// 	{
// 		date : {
// 			Value: "2022/04/16"
// 			Text: "04/16"
// 			Weekday: "土"
// 		},
// 		times: [
// 			{
// 				Value: "2022-04-16T19:00+09:00"
// 				Text: "19:00"
// 			},
// 			{
// 				Value: "2022-04-16T19:30+09:00"
// 				Text: "19:30"
// 			},
// 			{
// 				Value: "2022-04-16T19:00+09:00"
// 				Text: "20:00"
// 			},
// 		],
//
// 	},
// 	{
// 		date : {
// 			Value: "2022/04/17"
// 			Text: "04/17"
// 			Weekday: "土"
// 		},
// 		times: [
// 			{
// 				Value: "2022-04-17T19:00+09:00"
// 				Text: "19:00"
// 			},
// 			{
// 				Value: "2022-04-17T19:30+09:00"
// 				Text: "19:30"
// 			},
// 			{
// 				Value: "2022-04-17T19:00+09:00"
// 				Text: "20:00"
// 			}
// 		],
// 	}
// }
type FreeTimeSchedules []FreeTimeSchedule

type FreeTimeSchedule struct {
	FreeTimeDate FreeTimeDate `json:"date"`
	FreeTimes    []FreeTime   `json:"times"`
}

type FreeTimeDate struct {
	Value   string `json:"value"`
	Text    string `json:"text"`
	Weekday string `json:"weekday"`
}
type FreeTime struct {
	Value string `json:"value"`
	Text  string `json:"text"`
}

// Event calendarEventsからアプリ用に変換したもの
type Event struct {
	CalendarId    string
	CalendarName  string
	Title         string
	IsAllDay      bool
	StartDateTime time.Time
	EndDateTime   time.Time
}

var regularHolidayWeekdays = []time.Weekday{time.Wednesday, time.Thursday}

// sample calendar ids
var calendarIds = []string{
	"kg090637fo0f1lg5s3ham2bhk8@group.calendar.google.com",
	"0lqtb45e5rpi3jmvjs4kcrrh94@group.calendar.google.com",
	"7j4hmerqr14ptp98p6b5p3io2k@group.calendar.google.com",
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

	now := time.Now()
	// 今日 + 翌日のスケジュール(+1d) + 期間(+DaysRange d) + 翌日(+1d)からnano秒マイナスして0時直前を取得(-1 nano)
	datetimeMin := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.Local)
	datetimeMax := time.Date(now.Year(), now.Month(), now.Day()+2+DaysRange, 0, 0, 0, 0, time.Local).Add(-1 * time.Nanosecond)
	timeMin := datetimeMin.Format(time.RFC3339)
	timeMax := datetimeMax.Format(time.RFC3339)
	fmt.Println(timeMin)
	fmt.Println(timeMax)

	// 祝日のdateを保持する配列
	// ex: ["2022/04/29", "2022/05/02"]
	holidayDates := make([]string, 0, 0)
	holidayCalendarId := "ja.japanese#holiday@group.v.calendar.google.com"
	holidayCalendarEvents, err := calendarService.Events.List(holidayCalendarId).MaxResults(int64(250)).TimeMin(timeMin).TimeMax(timeMax).TimeZone(DefaultTimeZone).Do()
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range holidayCalendarEvents.Items {
		holidayDates = append(holidayDates, v.Start.Date)
	}

	for _, calendarId := range calendarIds {
		events, err := calendarService.Events.List(calendarId).MaxResults(int64(250)).TimeMin(timeMin).TimeMax(timeMax).TimeZone(DefaultTimeZone).Do()
		if err != nil {
			log.Fatal(err)
		}
		for _, item := range events.Items {
			event, err := NewEvent(calendarId, events.Summary, item.Summary, item)
			if err != nil {
				log.Fatal(err)
			}
			// log.Printf("calendar name: %v\ttitle: %+v\tstart: %+v\tend: %+v\n", event.CalendarName, event.Title, event.StartDateTime, event.EndDateTime)

			// "2022/04/16": 000000000000000000001111110001100001000110000000
			mapDateBits := make(map[string]uint64)
			if err := convertToBits(mapDateBits, event); err != nil {
				log.Fatal(err)
			}
			for date, v := range mapDateBits {
				// CalendarBitsがネストのため先に日付キーをチェックし、なければIDとbitsのkey:valueを入れる。
				if _, ok := CalendarBits[date]; !ok {
					CalendarBits[date] = map[string]uint64{calendarId: v}
					continue
				}
				// CalendarBitsがネストのため先にカレンダーIDのキーをチェックし、なければbitsのvalueを入れる。
				if _, ok := CalendarBits[date][calendarId]; !ok {
					CalendarBits[date][calendarId] = v
					continue
				}
				// 1日に複数イベントがある場合、日付キーとIDとキーがすでに存在している。
				// 以下のようにイベントのbitが流れてくるので論理和で集約する。
				// event_1: 0000000011
				// event_2: 0000110000
				//        → 0000110011
				// key:value型で表すと以下になる。
				// {"2022/04/18" : {"hoge@example.com": 0000110011}}
				CalendarBits[date][calendarId] |= v
			}
		}
	}
	// b, err := json.MarshalIndent(CalendarBits, "", "    ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// fmt.Println(string(b))

	// 上のコードではイベントがない日付を取得することができないため、
	// カレンダーID（ユーザー）ごとにごとに1つもイベントがない日はCalendarBitsにすべて0を入れることで空いていることにする。
	// ex: 2022/04/19のイベントがまったくないとき、CalendarBitsの2022/04/19のキーを作成した上ですべてに0を入れる。
	// {"2022/04/18": {"hoge@example.com": 0000000000001110...}, "2022/04/20": {"hoge@example.com": 0000001111100000...} }
	// -> {"2022/04/18": {"hoge@example.com": 000000100100000...},"2022/04/19": {"hoge@example.com": 0000000000000000...}, "2022/04/20": {"hoge@example.com": 0000001111100000...} }
	for _, id := range calendarIds {
		diffDates := datetimeMax.Sub(datetimeMin).Hours() / 24
		forIncrementDate := datetimeMin
		for i := 0; i <= int(diffDates); i++ {
			// 日付のキーがない = だれもイベントが入っていない
			if _, ok := CalendarBits[forIncrementDate.Format(FormatDate)]; !ok {
				CalendarBits[forIncrementDate.Format(FormatDate)] = map[string]uint64{id: 0 << 31}
				forIncrementDate = forIncrementDate.AddDate(0, 0, 1)
				continue
			}
			// 日付のキーがある = だれかのイベントが入っている。
			// -> ユーザー特定ですべてに0を埋める。
			if _, ok := CalendarBits[forIncrementDate.Format(FormatDate)][id]; !ok {
				CalendarBits[forIncrementDate.Format(FormatDate)][id] = 0 << 31
			}
			forIncrementDate = forIncrementDate.AddDate(0, 0, 1)
		}
	}

	// for date, v := range CalendarBits {
	// 	fmt.Printf("------ %v ------\n", date)
	// 	for email, bits := range v {
	// 		fmt.Printf("\t%v %064b\n", email, bits)
	// 	}
	// }

	// 日付:bits に集約する。
	// 誰か1人でも空きがあれば予定が空いている仕様。
	// keyがない場合に論理積で集約するとすべて0になる。間違いの例: https://go.dev/play/p/Vb-RNavLeHc
	// 最初は論理和で集約し、2つ目から論理積で集約する。 https://go.dev/play/p/psnhye-ZZKp
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
	// for date, v := range dateBits {
	// 	fmt.Printf("///////// %v /////////\n", date)
	// 	fmt.Printf("%064b\n", v)
	// }

	// webサーバーと仮定し、レスポンス用で見やすい形に成形する。
	// 論理積で集約したbitsを空き時間枠に変換する。
	displayToFreeBusyCalendar := make(FreeTimeSchedules, 0, DaysRange)
	for strDate, bits := range dateBits {
		date, _ := time.Parse(FormatDate, strDate)
		// fmt.Printf("-------%v-------\n", i)

		calendarDate := FreeTimeDate{Value: date.Format(FormatDate), Text: date.Format("01/02"), Weekday: date.Weekday().String()}
		bt := FreeTimeSchedule{
			FreeTimeDate: calendarDate,
			FreeTimes:    make([]FreeTime, 0, EndMaxTimeHour*2), // 仮で30分枠で*2している。
		}

		// 休日、祝日の場合は空いていないという形に変換する。
		// Todo: そもそもbit換算時に全部1にできると良いかも
		isHoliday := false
		for _, holiday := range regularHolidayWeekdays {
			if holiday == date.Weekday() {
				isHoliday = true
			}
		}
		for _, holiday := range holidayDates {
			if holiday == date.Format("2006-01-02") {
				isHoliday = true
			}
		}
		// times(FreeTimes)のみ空で返すでOK
		if isHoliday {
			displayToFreeBusyCalendar = append(displayToFreeBusyCalendar, bt)
			continue
		}

		// 開始時間〜終了時間のbit位置（30分枠で*2している）でループを実行
		// Todo: 1時間枠/15分枠の対応
		for i := uint(StartMinTimeHour * 2); i < uint(EndMaxTimeHour*2); i++ {
			// fmt.Println(v & (1 << i))

			// bits&(1<<i) != 1<<iについて
			// 右からi番目のbitを1にしたときの値 と bitsの論理積を取得し、
			// その時間枠が1である（右オペランドの1<<iで右からi番目のbitが1かどうかを見ている）かを条件分岐
			// もし1であれば予定ありなのでなにもしない → FreeTime構造体は空で返す。
			// もし1でなければ（0であれば）、予定なしなので、空き時間をFreeTime構造体にビルドする。
			if bits&(1<<i) != 1<<i {
				// 右からi番目を時間枠の分割（1つの時間枠が30分なら 60分 / 30分 = 2）で除算する。
				// 右から8番目が0であれば、 8 / (60/30) = 8 / 2 = 4 （4時台）
				// 右から9番目が0であれば、 9 / (60/30) = 9 / 2 = 4 （4時台）
				hour := i / (60 / EventTimeFrameMinutes)
				// 右からi番目を時間枠の分割（1つの時間枠が30分なら 60分 / 30分 = 2）の余りに1つの時間枠をかける
				// 右から8番目が0であれば、 8 % (60/30) * 30 = 8 % 2 * 30 = 0 * 30 = 0
				// 右から9番目が0であれば、 9 % (60/30) * 30 = 9 % 2 * 30 = 1 * 30 = 30
				minute := i % (60 / EventTimeFrameMinutes) * EventTimeFrameMinutes
				// → 右から8/9番目が0であれば04:00~04:30 / 04:30 ~ 05:00までが空きだということになる。
				freeTime := time.Date(date.Year(), date.Month(), date.Day(), int(hour), int(minute), 0, 0, time.Local)
				// fmt.Println(freeTime)
				calendarTime := FreeTime{Value: freeTime.Format(time.RFC3339), Text: freeTime.Format("15:04")}
				bt.FreeTimes = append(bt.FreeTimes, calendarTime)
			}
		}
		displayToFreeBusyCalendar = append(displayToFreeBusyCalendar, bt)
		// fmt.Printf("-------%v-------\n", i)
	}

	b, err := json.MarshalIndent(displayToFreeBusyCalendar, "", "    ")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(b))

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
	var isAllDay bool
	sTime, eTime, err := timeParseRangeRFC3339(item.Start.DateTime, item.End.DateTime)
	if err != nil {
		// all-day（終日）イベントであればevent.Start.Dateに値が入る
		// see: https://pkg.go.dev/google.golang.org/api/calendar/v3#EventDateTime
		sTime, eTime, err = timeParseRangeRFC3339(item.Start.Date+"T00:00:00+09:00", item.End.Date+"T23:59:59+09:00")
		if err != nil {
			return nil, err
		}
		isAllDay = true
	}
	return &Event{CalendarId: id, CalendarName: name, Title: title, IsAllDay: isAllDay, StartDateTime: sTime, EndDateTime: eTime}, nil
}

// convertToBits  key:日付 value:bit換算の予定
// Todo: 営業時間枠のみのbitを用意する
// Todo: 15分刻みの時間枠対応
func convertToBits(mapDateBits map[string]uint64, event *Event) error {
	// 終日イベントの場合は日にちごとに分割してすべて1となるbitの群をなす。
	if event.IsAllDay {
		// ex:
		// Startが2022/04/18 / Endが2022/04/23の終日イベントの場合
		// -> 2022/04/18, 2022/04/19, 2022/04/20, 2022/04/21, 2022/04/22, 2022/04/23 に分割。
		// diffDays -> 120h → 120 / 24 -> 2022/04/18から2022/04/23まで+5日の差分
		diffDays := event.EndDateTime.Sub(event.StartDateTime).Hours() / 24
		fotIncrementDate := event.StartDateTime
		for i := 0; i <= int(diffDays); i++ {
			mapDateBits[fotIncrementDate.Format(FormatDate)] = 0b11111111111111111111111111111111
			fotIncrementDate = fotIncrementDate.AddDate(0, 0, 1)
		}
		return nil
	}

	// ex1. 30分ごとに分割する & 08:00 ~ 09:00の予定の場合
	// ...00 11 00 00 00 00 00 00 00 00 （右から00:00~00:30とカウント）
	// 右から17bit目が、08:00~08:30
	// 右から18bit目が、08:30~09:00

	// ex2. 30分ごとに分割する & 08:30 ~ 09:30の予定の場合
	// ...01 10 00 00 00 00 00 00 00 00
	// 右から19bit目が、08:30~09:00
	// 右から20bit目が、09:00~09:30

	// 予定開始のbit位置を取得
	// startTimeBitは上記例の場合16という数字を得られるが、bit処理する際に17番目が1になる。
	// 1 << 16 = 17bit目に1が立つ（1 << 0 で1bit目）
	// sample: https://go.dev/play/p/jkBDUwnxWMQ
	// Todo: 15分刻みの対応
	startTimeBit := uint64(event.StartDateTime.Hour() * (60 / EventTimeFrameMinutes))
	if event.StartDateTime.Minute() >= EventTimeFrameMinutes {
		startTimeBit++
	}
	// 予定終了のbit位置を取得
	// 30分枠のとき、 ~ 03:00 までで 3(時) * 2 -> 6が取れるが、bit換算の際に 1 << 6 をすると、
	// 7bit目が1になってしまいずれる -> 6bit目に1を立てたいので-1する。
	// 00 10 00 00 -> i << 5
	// https://go.dev/play/p/KNkmnPrSd0K
	// Todo: 15分刻みの対応
	endTimeBit := uint64(event.EndDateTime.Hour() * (60 / EventTimeFrameMinutes))
	if event.EndDateTime.Minute() == 0 {
		endTimeBit--
	} else if event.EndDateTime.Minute() > EventTimeFrameMinutes {
		endTimeBit++
	}

	// 時間枠をbitに換算する。
	date := event.StartDateTime.Format(FormatDate)

	// 論理和で集約していく。
	// ex1: 01:00~02:00に予定がある場合、右から3bit目と4bit目を1にする。（右から1bit目は 00:00 ~ 00:30）
	// -> startTimeBit = 2 （1 << 2 → 0100）
	// -> endTimeBit = 3 （1 << 3 → 1000）
	// 論理和集約 -> 1100
	// ex1: 02:00~03:30に予定がある場合 -> 01110000
	// -> startTimeBit = 4 （1 << 2 → 0100）
	// -> endTimeBit = 6 （1 << 3 → 1000）
	// 論理和集約 -> 1100
	// 論理和のexample: https://go.dev/play/p/0D36wM4fVxt
	for i := startTimeBit; i <= endTimeBit; i++ {
		mapDateBits[date] |= 1 << i
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
