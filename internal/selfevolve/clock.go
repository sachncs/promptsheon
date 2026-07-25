package selfevolve

import "time"

// fallbackTS is a function variable so the test can
// override the clock. Production leaves it as time.Now.
var fallbackTS = func() int64 { return time.Now().UnixNano() }
