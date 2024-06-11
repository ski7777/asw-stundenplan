package main

import (
	"errors"
	"fmt"
	"github.com/akamensky/argparse"
	ics "github.com/arran4/golang-ical"
	"github.com/ski7777/asw-stundenplan/pkg/extended_event"
	"github.com/ski7777/asw-stundenplan/pkg/roomheatmap"
	"github.com/ski7777/asw-stundenplan/pkg/rooms"
	"github.com/ski7777/asw-stundenplan/pkg/timetablelist"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetablepage"
	"github.com/thoas/go-funk"
	"log"
	"os"
	"path"
	"strings"
	"sync"
	"time"
)

func main() {
	parser := argparse.NewParser("asw-stundenplan", "scrapes all timetable events from ASW gGmbH ans saves them as ics")
	outputdir := parser.String("o", "out", &argparse.Options{Required: false, Help: "output directory", Default: "out"})
	timezone := parser.String("t", "timezone", &argparse.Options{Required: false, Help: "timezone", Default: "Europe/Berlin"})
	interval := parser.Int("i", "interval", &argparse.Options{Required: false, Help: "interval", Default: nil})
	motdSummary := parser.String("m", "motd-summary", &argparse.Options{Required: false, Help: "motd summary", Default: ""})
	motdDescription := parser.StringList("d", "motd-description", &argparse.Options{Required: false, Help: "motd description", Default: []string{}})
	movingEventYears := parser.Int("n", "moving-event", &argparse.Options{Required: false, Help: "moving event n years in the future. 0 disables this feature", Default: 0})
	movingEventDescription := parser.String("e", "moving-event-description", &argparse.Options{Required: false, Help: "moving event description. use %d as placeholder for the years"})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}
	if _, err := os.Stat(*outputdir); os.IsNotExist(err) {
		log.Fatalln("output directory does not exist")
	}
	if err := os.MkdirAll(path.Join(*outputdir, "rooms"), 0755); err != nil {
		log.Fatalln(err)
	}
	tz, err := time.LoadLocation(*timezone)
	if err != nil {
		log.Fatalln(err)
	}
	if interval == nil || *interval == 0 {
		run(tz, *outputdir, motdSummary, motdDescription, *movingEventYears, movingEventDescription)
	} else {
		log.Println(fmt.Sprintf("running in interval mode. Interval %d seconds", *interval))
		ticker := time.NewTicker(time.Duration(*interval) * time.Second)
		runing := false
		runOnce := func() {
			if runing {
				return
			}
			runing = true
			run(tz, *outputdir, motdSummary, motdDescription, *movingEventYears, movingEventDescription)
			runing = false
		}
		runOnce()
		go func() {
			for {
				select {
				case <-ticker.C:
					runOnce()
				}
			}
		}()
		<-make(chan struct{})
	}
}

func run(tz *time.Location, outputdir string, motdSummary *string, motdDescription *[]string, movingEventYears int, movingEventDescription *string) {
	log.Println("scraping timetable urls")
	ttm, err := timetablelist.GetTimeTableListDefault()
	if err != nil {
		log.Fatalln(err)
	}
	log.Println(
		fmt.Sprintf(
			"found %d classes with a total of %d timetablepages",
			len(ttm),
			funk.Reduce(
				funk.Values(ttm),
				func(acc int, cttpm timetablelist.ClassTimeTablePagesMap) int {
					return acc + len(cttpm)
				},
				0,
			),
		),
	)
	var (
		errorsMutex  = &sync.Mutex{}
		threadErrors []error
		wg           sync.WaitGroup
		eventsMutex  = &sync.Mutex{}
		events       = make(map[string]map[string]*extended_event.ExtendedEvent) //class --> {id --> event}
	)
	log.Println("scraping all timetablepages")
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
				var classEvents map[string]*extended_event.ExtendedEvent
				var ok bool
				if classEvents, ok = events[cn]; !ok {
					classEvents = make(map[string]*extended_event.ExtendedEvent)
				}
				defer func() { events[cn] = classEvents }()
				for id, e := range ttp.AllEvents {
					if _, ok := classEvents[id]; ok {
						defer errorsMutex.Unlock()
						errorsMutex.Lock()
						threadErrors = append(threadErrors, errors.New(fmt.Sprintf("duplicate event id: class: %s, block: %d, event: %s", cn, bn, id)))
						return
					}
					classEvents[id] = extended_event.NewExtendedEvent(e, id, cn, tz)
				}

			}(cn, bn, tturl)
		}
	}
	wg.Wait()
	if len(threadErrors) > 0 {
		for _, e := range threadErrors {
			log.Println(e)
		}
		log.Fatalln("exiting due to errors above")
	}
	log.Println(
		fmt.Sprintf(
			"scraped %d events for %d classes",
			funk.Reduce(
				funk.Values(events),
				func(acc int, cem map[string]*extended_event.ExtendedEvent) int {
					return acc + len(cem)
				},
				0,
			),
			len(events),
		),
	)
	log.Println("writing ics files")
	now := time.Now()
	var motd, movingEvent *ics.VEvent
	if *motdSummary != "" {
		motd = ics.NewEvent("motd")
		motd.SetSummary(*motdSummary)
		motd.SetDtStampTime(now)
		motd.SetAllDayStartAt(now)
		motd.SetAllDayEndAt(now)
		motd.SetTimeTransparency(ics.TransparencyOpaque)
		motd.SetStatus(ics.ObjectStatusTentative)
		motd.SetColor("red")
		if len(*motdDescription) > 0 {
			motd.SetDescription(strings.Join(*motdDescription, "\n"))
		}
	}
	if movingEventYears > 0 {
		movingDate := now.AddDate(movingEventYears, 0, 0)
		movingEvent = ics.NewEvent("moving-event")
		movingEvent.SetSummary("moving-event")
		if movingEventDescription != nil {
			movingEvent.SetDescription(fmt.Sprintf(*movingEventDescription, movingEventYears))
		}
		movingEvent.SetDtStampTime(movingDate)
		movingEvent.SetAllDayStartAt(movingDate)
		movingEvent.SetAllDayEndAt(movingDate)
		movingEvent.SetTimeTransparency(ics.TransparencyTransparent)
		movingEvent.SetStatus(ics.ObjectStatusTentative)
		movingEvent.SetColor("red")
	}
	for cn, ce := range events {
		wg.Add(1)
		go func(cn string, ce map[string]*extended_event.ExtendedEvent) {
			defer wg.Done()
			cal := ics.NewCalendarFor(cn)
			cal.SetXPublishedTTL("PT10M")
			cal.SetRefreshInterval("PT10M")
			cal.SetMethod(ics.MethodRequest)
			cal.SetTzid(tz.String())
			cal.SetTimezoneId(tz.String())
			cal.SetLastModified(now)
			cal.SetProductId(fmt.Sprintf("Stundenplan für die Klasse %s der ASW gGmbH.", cn))
			cal.SetDescription("Weitere Informationen zum Stundenplan finden Sie unter https://github.com/ski7777/asw-stundenplan")
			for _, e := range ce {
				cal.AddVEvent(e.ToVEvent())
			}
			if motd != nil {
				cal.AddVEvent(motd)
			}
			if movingEvent != nil {
				cal.AddVEvent(movingEvent)
			}
			af, err := os.Create(path.Join(outputdir, cn+".ics"))
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
	if len(threadErrors) > 0 {
		for _, e := range threadErrors {
			log.Println(e)
		}
		log.Fatalln("exiting due to errors above")
	}
	log.Println("generating room heatmaps")
	for rhmd := 0; rhmd < 14; rhmd++ {
		go func(rhmd int) {
			rhmstart := time.Date(
				now.Year(),
				now.Month(),
				now.Day(),
				0,
				0,
				0,
				0,
				tz,
			).Add(time.Hour * 24 * time.Duration(rhmd))
			rhmend := rhmstart.Add(time.Hour * 24) // 1 day
			rhm := roomheatmap.NewRoomHeatmap(
				rhmstart,
				rhmend,
				time.Minute*15,
				rooms.Rooms,
				tz,
				funk.Reduce(funk.Values(events), func(acc []*extended_event.ExtendedEvent, ae map[string]*extended_event.ExtendedEvent) []*extended_event.ExtendedEvent {
					return append(acc, funk.Values(ae).([]*extended_event.ExtendedEvent)...)
				}, []*extended_event.ExtendedEvent{}).([]*extended_event.ExtendedEvent),
			)
			rhm.MinTime = time.Date(0, 0, 0, 8, 0, 0, 0, tz)
			rhm.MaxTime = time.Date(0, 0, 0, 20, 0, 0, 0, tz)

			af, err := os.Create(path.Join(outputdir, "rooms", fmt.Sprintf("%02d.html", rhmd)))
			if err != nil {
				threadErrors = append(threadErrors, err)
				return
			}
			defer func(af *os.File) {
				_ = af.Close()
			}(af)
			var rhmhtml string
			rhmhtml, err = rhm.GenHTML()
			if err != nil {
				threadErrors = append(threadErrors, err)
				return
			}
			_, err = af.WriteString(rhmhtml)
			if err != nil {
				threadErrors = append(threadErrors, err)
				return
			}
		}(rhmd)
	}
	wg.Wait()
	if len(threadErrors) > 0 {
		for _, e := range threadErrors {
			log.Println(e)
		}
		log.Fatalln("exiting due to errors above")
	}
	log.Println("done")
}
