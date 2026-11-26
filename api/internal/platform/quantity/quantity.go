// Package quantity writes quantities in thousandths of a unit (ADR-0009) for
// people, the way the app does.
package quantity

import (
	"fmt"
	"strconv"
	"strings"
)

// names are the unit names in pt-PT, singular and plural.
var names = map[string][2]string{
	"kg": {"kg", "kg"}, "l": {"L", "L"}, "un": {"unidade", "unidades"},
	"molho": {"molho", "molhos"}, "duzia": {"dúzia", "dúzias"},
}

// Format writes milli thousandths of unit as "12,5 kg", "2 L", "3 dúzias" or
// "1 molho".
func Format(milli int64, unit string) string {
	whole, frac := milli/1000, milli%1000
	if frac < 0 {
		frac = -frac
	}
	n := strconv.FormatInt(whole, 10)
	if frac != 0 {
		if whole == 0 && milli < 0 {
			n = "-0"
		}
		n += "," + strings.TrimRight(fmt.Sprintf("%03d", frac), "0")
	}
	name, ok := names[unit]
	if !ok {
		return n + " " + unit
	}
	if milli == 1000 || milli == -1000 {
		return n + " " + name[0]
	}
	return n + " " + name[1]
}
