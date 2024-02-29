package extended_event

import (
	"fmt"
	ics "github.com/arran4/golang-ical"
	"github.com/ski7777/asw-stundenplan/pkg/rooms"
	"github.com/ski7777/sked-campus-html-parser/pkg/timetable"
	"github.com/thoas/go-funk"
	"strings"
	"time"
)

type ExtendedEvent struct {
	Event      timetable.Event
	ID         string
	Begin, End time.Time
	Class      string

	timezone                       *time.Location
	Summary, Location, Description *string
	timeTransparency               ics.TimeTransparency
	status                         ics.ObjectStatus
}

func (ee *ExtendedEvent) ToVEvent() *ics.VEvent {
	ie := ics.NewEvent(ee.ID)
	ie.SetDtStampTime(time.Now())
	ie.SetStartAt(ee.Begin)
	ie.SetEndAt(ee.End)
	if ee.Summary != nil {
		ie.SetSummary(*ee.Summary)
	}
	if ee.Location != nil {
		ie.SetLocation(*ee.Location)
	}
	if ee.Description != nil {
		ie.SetDescription(*ee.Description)
	}
	ie.SetTimeTransparency(ee.timeTransparency)
	ie.SetStatus(ee.status)
	return ie
}

func (ee *ExtendedEvent) GetRooms() (rl []string) {
	rl = []string{}
	if ee.Location == nil {
		return
	}
	for _, match := range rooms.RoomRegex.FindAllString(*ee.Location, -1) {
		rl = append(rl, match)
	}
	return
}

func (ee *ExtendedEvent) String() string {
	var data = []string{ee.Class}
	if ee.Summary != nil {
		data = append(data, *ee.Summary+":")
	}
	data = append(data, ee.Begin.Format("15:04")+"-"+ee.End.Format("15:04"))
	if ee.Location != nil {
		data = append(data, "Ort:"+*ee.Location)
	}
	if ee.Description != nil {
		data = append(data, *ee.Description)
	}
	return strings.Join(data, "\n")
}

func NewExtendedEvent(se timetable.Event, id string, cn string, timezone *time.Location) *ExtendedEvent {
	ee := &ExtendedEvent{
		Event:            se,
		ID:               id,
		Class:            cn,
		timezone:         timezone,
		Begin:            se.Begin.ToTime(se.Date, timezone),
		End:              se.End.ToTime(se.Date, timezone),
		timeTransparency: ics.TransparencyOpaque,
		status:           ics.ObjectStatusConfirmed,
	}
	if len(se.Text) > 0 {
		ee.Summary = &se.Text[0]
		if se.Text[0] == "Reserviert" {
			ee.timeTransparency = ics.TransparencyTransparent
			ee.status = ics.ObjectStatusTentative
		}
	}
	if len(se.Text) > 1 {
		*ee.Summary = fmt.Sprintf("%s (%s)", se.Text[1], se.Text[0])
		var locline string
		if roomlines := funk.FilterString(se.Text, func(s string) bool {
			return strings.HasPrefix(s, "NK: ") || strings.HasPrefix(s, "EXT: ")
		}); len(roomlines) > 0 {
			locline = roomlines[0]
			loc := locline
			loc = strings.ReplaceAll(loc, "NK: ", "")
			loc = strings.ReplaceAll(loc, "DV ", "")
			ee.Location = &loc
		}
		if desclines := funk.FilterString(se.Text[2:], func(s string) bool {
			return s != locline
		}); len(desclines) > 0 {
			var desc string
			desc = strings.Join(desclines, "\n")
			ee.Description = &desc
		}
	}
	return ee
}
