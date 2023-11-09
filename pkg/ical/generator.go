package ical

import (
	"fmt"
	"github.com/arran4/golang-ical"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetable"
	"github.com/thoas/go-funk"
	"strings"
	"time"
)

func ConvertEvent(se timetable.Event, id string, timezone *time.Location) (ie *ics.VEvent) {
	ie = ics.NewEvent(id)
	ie.SetDtStampTime(time.Now())
	ie.SetStartAt(se.Begin.ToTime(se.Date, timezone))
	ie.SetEndAt(se.End.ToTime(se.Date, timezone))
	var timeTransparency = ics.TransparencyOpaque
	var objectStatus = ics.ObjectStatusConfirmed
	if len(se.Text) > 0 {
		ie.SetSummary(se.Text[0])
		if se.Text[0] == "Reserviert" {
			timeTransparency = ics.TransparencyTransparent
			objectStatus = ics.ObjectStatusTentative
		}
	}
	if len(se.Text) > 1 {
		ie.SetSummary(fmt.Sprintf("%s (%s)", se.Text[1], se.Text[0]))
		var locline string
		if roomlines := funk.FilterString(se.Text, func(s string) bool {
			return strings.HasPrefix(s, "NK: ") || strings.HasPrefix(s, "EXT: ")
		}); len(roomlines) > 0 {
			locline = roomlines[0]
			loc := locline
			loc = strings.ReplaceAll(loc, "NK: ", "")
			loc = strings.ReplaceAll(loc, "DV ", "")
			ie.SetLocation(loc)
		}
		if desclines := funk.FilterString(se.Text[2:], func(s string) bool {
			return s != locline
		}); len(desclines) > 0 {
			ie.SetDescription(strings.Join(desclines, "\n"))
		}

	}
	ie.SetTimeTransparency(timeTransparency)
	ie.SetStatus(objectStatus)
	return
}
