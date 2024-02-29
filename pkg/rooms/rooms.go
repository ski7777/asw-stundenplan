package rooms

import "regexp"

var RoomRegex = regexp.MustCompile(`(?m)(\d\.\d{2})|PS`)

// these are the rooms that are usually used and should be shown in all diagrams
var Rooms = []string{
	"PS",
	"1.01",
	"1.02",
	"1.05",
	"1.06",
	"1.07",
	"1.09",
	"1.10",
	"1.11",
	"1.12",
	"2.04",
	"2.07",
	"2.08",
	"2.09",
	"2.11",
	"2.12",
	"2.13",
	"2.14",
	"3.09",
	"3.10",
	"3.11",
	"3.14",
	"3.15",
	"3.16",
}
