package cronet

// #include <stdlib.h>
// #include <stdbool.h>
// #include <cronet_c.h>
import "C"
import "time"

// DateTime
// Represents a date and time expressed as the number of milliseconds since the UNIX epoch.
type DateTime struct {
	ptr C.Cronet_DateTimePtr
}

func NewDateTime() DateTime {
	return DateTime{C.Cronet_DateTime_Create()}
}

func (t DateTime) Destroy() {
	C.Cronet_DateTime_Destroy(t.ptr)
}

// SetValue
// Number of milliseconds since the UNIX epoch.
func (t DateTime) SetValue(value time.Time) {
	C.Cronet_DateTime_value_set(t.ptr, C.int64_t(value.UnixMilli()))
}

func (t DateTime) Value() time.Time {
	return time.UnixMilli(int64(C.Cronet_DateTime_value_get(t.ptr)))
}
