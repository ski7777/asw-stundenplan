package main

import (
	"errors"
	"fmt"
	ics "github.com/arran4/golang-ical"
	"github.com/ski7777/asw-stundenplan/pkg/ical"
	"github.com/ski7777/asw-stundenplan/pkg/timetablelist"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetable"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetablepage"
	"log"
	"os"
	"path"
	"sync"
	"time"
)

func main() {
	ttm, err := timetablelist.GetTimeTableListDefault()
	if err != nil {
		log.Fatalln(err)
	}
	var (
		errorsMutex  = &sync.Mutex{}
		threadErrors []error
		wg           sync.WaitGroup
		eventsMutex  = &sync.Mutex{}
		events       = make(map[string]map[string]timetable.Event) //class --> {id --> event}
	)
	for cn, cttm := range ttm {
		for bn, tturl := range cttm {
			wg.Add(1)
			go func(cn string, bn int, tturl string) {
				defer wg.Done()
				var ttp *timetablepage.TimeTablePage
				var ierr error
				if ttp, ierr = timetablepage.ParseHTMLURL(tturl); ierr != nil {
					defer errorsMutex.Unlock()
					errorsMutex.Lock()
					threadErrors = append(threadErrors, ierr)
					return
				}
				eventsMutex.Lock()
				defer eventsMutex.Unlock()
				var classEvents map[string]timetable.Event
				var ok bool
				if classEvents, ok = events[cn]; !ok {
					classEvents = make(map[string]timetable.Event)
				}
				defer func() { events[cn] = classEvents }()
				for id, e := range ttp.AllEvents {
					if _, ok := classEvents[id]; ok {
						defer errorsMutex.Unlock()
						errorsMutex.Lock()
						threadErrors = append(threadErrors, errors.New(fmt.Sprintf("duplicate event id: class: %s, block: %d, event: %s", cn, bn, id)))
						return
					}
					classEvents[id] = e
				}

			}(cn, bn, tturl)
		}
	}
	wg.Wait()
	for _, e := range threadErrors {
		log.Println(e)
		log.Fatalln("exiting due to errors above")
	}
	tz, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		log.Fatalln(err)
	}
	for cn, ce := range events {
		wg.Add(1)
		go func(cn string, ce map[string]timetable.Event) {
			defer wg.Done()
			cal := ics.NewCalendarFor(cn)
			cal.SetXPublishedTTL("PT10M")
			cal.SetRefreshInterval("PT10M")
			cal.SetMethod(ics.MethodRequest)
			for id, e := range ce {
				cal.AddVEvent(ical.ConvertEvent(e, id, tz))
			}
			af, err := os.Create(path.Join("out", cn+".ics"))
			if err != nil {
				log.Fatalln(err)
			}
			defer func(af *os.File) {
				_ = af.Close()
			}(af)
			ierr := cal.SerializeTo(af)
			if err != nil {
				defer errorsMutex.Unlock()
				errorsMutex.Lock()
				threadErrors = append(threadErrors, ierr)
				return
			}
		}(cn, ce)
	}
	wg.Wait()
	for _, e := range threadErrors {
		log.Println(e)
		log.Fatalln("exiting due to errors above")
	}
}
