package observability

import "time"

type Recorder interface {
    Count(name string, value int64, dimensions map[string]string)
    Duration(name string, value time.Duration, dimensions map[string]string)
}
