package timetablelist

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/ski7777/asw-stundenplan/internal/tools"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
)

var ttnamematcher = regexp.MustCompile(`(?P<class>.+)-(?P<block>[[:digit:]])\.Block`)

type ClassTimeTableMap map[int]string
type TimeTableMap map[string]ClassTimeTableMap

func GetTimeTableList(overviewurl string, selector string) (data TimeTableMap, err error) {
	res, err := http.Get(overviewurl)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != 200 {
		return nil, errors.New(fmt.Sprintf("status code error: %d %s", res.StatusCode, res.Status))
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	parsedurl, err := url.Parse(overviewurl)
	if err != nil {
		log.Fatal(err)
	}

	data = make(TimeTableMap)
	doc.Find(selector).EachWithBreak(func(_ int, s *goquery.Selection) (coninue bool) {
		ttname := s.Text()
		if len(ttname) == 0 {
			return
		}
		if href, hrefok := s.Attr("href"); hrefok {
			if parsedurl.Path, err = url.QueryUnescape(href); err != nil {
				return
			} else {
				if matches := tools.FindNamedMatches(ttnamematcher, ttname); matches == nil {
					err = errors.New(fmt.Sprintf("failed to parse timetable name: %s", ttname))
					return
				} else {
					if class, classok := matches["class"]; !classok {
						err = errors.New(fmt.Sprintf("failed to parse timetable name: %s", ttname))
						return
					} else {
						if block, blockok := matches["block"]; !blockok {
							err = errors.New(fmt.Sprintf("failed to parse timetable name: %s", ttname))
							return
						} else {
							var blockint int
							if blockint, err = strconv.Atoi(block); err != nil {
								err = errors.New(fmt.Sprintf("failed to parse timetable name: %s", ttname))
								return
							} else {
								if _, ok := data[class]; !ok {
									data[class] = make(ClassTimeTableMap)
								}
								data[class][blockint] = parsedurl.String()
							}
						}
					}
				}
			}
		}
		return true
	})
	return
}

func GetTimeTableListDefault() (data TimeTableMap, err error) {
	return GetTimeTableList(
		"https://www.asw-ggmbh.de/laufender-studienbetrieb/stundenplaene",
		".table-responsive a.verweis",
	)
}
