package main

import (
	"errors"
	"fmt"
	"github.com/akamensky/argparse"
	ics "github.com/arran4/golang-ical"
	"github.com/ski7777/asw-stundenplan/pkg/ical"
	"github.com/ski7777/asw-stundenplan/pkg/timetablelist"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetable"
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
	motdDescription := parser.StringList("d", "motd-description", &argparse.Options{Required: false, Help: "motd description", Default: ""})
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Print(parser.Usage(err))
	}
	if _, err := os.Stat(*outputdir); os.IsNotExist(err) {
		log.Fatalln("output directory does not exist")
	}
	tz, err := time.LoadLocation(*timezone)
	if err != nil {
		log.Fatalln(err)
	}
	if interval == nil || *interval == 0 {
		run(tz, *outputdir, motdSummary, motdDescription)
	} else {
		log.Println(fmt.Sprintf("running in interval mode. Interval %d seconds", *interval))
		ticker := time.NewTicker(time.Duration(*interval) * time.Second)
		runing := false
		runOnce := func() {
			if runing {
				return
			}
			runing = true
			run(tz, *outputdir, motdSummary, motdDescription)
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

func run(tz *time.Location, outputdir string, motdSummary *string, motdDescription *[]string) {
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
		events       = make(map[string]map[string]timetable.Event) //class --> {id --> event}
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
				func(acc int, cem map[string]timetable.Event) int {
					return acc + len(cem)
				},
				0,
			),
			len(events),
		),
	)
	log.Println("writing ics files")
	now := time.Now()
	var motd *ics.VEvent
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
	for cn, ce := range events {
		wg.Add(1)
		go func(cn string, ce map[string]timetable.Event) {
			defer wg.Done()
			cal := ics.NewCalendarFor(cn)
			cal.SetXPublishedTTL("PT10M")
			cal.SetRefreshInterval("PT10M")
			cal.SetMethod(ics.MethodRequest)
			cal.SetTzid(tz.String())
			cal.SetTimezoneId(tz.String())
			cal.SetLastModified(now)
			for id, e := range ce {
				cal.AddVEvent(ical.ConvertEvent(e, id, tz))
			}
			if motd != nil {
				cal.AddVEvent(motd)
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
	log.Println("done")
}
