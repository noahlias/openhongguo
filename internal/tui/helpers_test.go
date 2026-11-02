package tui

import (
	"fmt"
	"reflect"
	"time"
)

func waitABit() { time.Sleep(5 * time.Millisecond) }

// emptyField names the first empty string field of a Strings value.
func emptyField(s Strings) string {
	v := reflect.ValueOf(s)
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).String() == "" {
			return fmt.Sprintf("Strings.%s is empty", v.Type().Field(i).Name)
		}
	}
	return ""
}
