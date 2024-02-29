package roomheatmap

import (
	"bytes"
	"fmt"
	"github.com/ski7777/asw-stundenplan/pkg/extended_event"
	"github.com/thoas/go-funk"
	"sort"
	"time"
)

type RoomHeatmap struct {
	Interval         time.Duration
	Start            time.Time
	End              time.Time
	Rooms            []string
	Slots            []map[string][]*extended_event.ExtendedEvent // [Slot Index](Room Name -> []Event)
	MinTime, MaxTime time.Time
}

func (r *RoomHeatmap) AddEvents(el []*extended_event.ExtendedEvent) {
	for _, e := range el {
		r.AddEvent(e)
	}
}

func (r *RoomHeatmap) AddEvent(e *extended_event.ExtendedEvent) {
	if e.Begin.After(r.End) || e.End.Before(r.Start) {
		return
	}
	startslot := int(e.Begin.Sub(r.Start) / r.Interval)
	endslot := int((e.End.Sub(r.Start) - time.Second) / r.Interval)
	if startslot < 0 {
		startslot = 0
	}
	if endslot >= len(r.Slots) {
		endslot = len(r.Slots) - 1
	}
	rooms := funk.IntersectString(e.GetRooms(), r.Rooms)
	for sid := startslot; sid <= endslot; sid++ {
		for _, room := range rooms {
			if _, ok := r.Slots[sid][room]; !ok {
				continue
			}
			r.Slots[sid][room] = append(r.Slots[sid][room], e)
		}
	}
}

func (r *RoomHeatmap) GenHTML() (string, error) {
	type slot struct {
		X    string `json:"x"` //Time
		Y    string `json:"y"` //Room
		Heat int    `json:"heat"`
		Text string `json:"text"`
	}
	var data []slot
	for sid, s := range r.Slots {
		t := r.Start.Add(time.Duration(int(r.Interval) * sid))
		daytime := time.Date(0, 0, 0, t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), t.Location())
		if daytime.Before(r.MinTime) || daytime.Add(time.Second).After(r.MaxTime) {
			continue
		}
		for room, events := range s {
			data = append(data, slot{
				X:    t.Format("02.01. 15:04"),
				Y:    room,
				Heat: len(events),
				Text: func() (text string) {
					for i, e := range events {
						if len(events) > 1 {
							text += fmt.Sprintf("Veranstaltung %d:\n", i+1)
						}
						text += e.String()
						if i < len(events)-1 {
							text += "\n\n"
						}
					}
					return
				}(),
			})
		}
	}

	sort.Slice(data, func(i, j int) bool {
		if data[i].X != data[j].X {
			return data[i].X < data[j].X
		} else {
			return data[i].Y < data[j].Y
		}
	})

	var buf bytes.Buffer
	err := rhm_template.Execute(&buf, struct {
		Data  []slot
		Rooms []string
	}{
		Data:  data,
		Rooms: r.Rooms,
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func NewRoomHeatmap(start time.Time, end time.Time, interval time.Duration, rooms []string, tz *time.Location, events []*extended_event.ExtendedEvent) *RoomHeatmap {
	roomHeatmap := &RoomHeatmap{
		Interval: interval,
		Start:    start,
		End:      end,
		Slots:    make([]map[string][]*extended_event.ExtendedEvent, end.Sub(start)/interval),
		Rooms:    rooms,
		MinTime:  time.Date(0, 0, 0, 0, 0, 0, 0, tz),
		MaxTime:  time.Date(0, 0, 0, 24, 0, 0, 0, tz),
	}

	sort.Strings(roomHeatmap.Rooms)

	for i := range roomHeatmap.Slots {
		roomHeatmap.Slots[i] = make(map[string][]*extended_event.ExtendedEvent)
		for _, room := range rooms {
			roomHeatmap.Slots[i][room] = []*extended_event.ExtendedEvent{}
		}
	}

	roomHeatmap.AddEvents(events)

	return roomHeatmap
}
