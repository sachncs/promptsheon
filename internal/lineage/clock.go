package lineage

import "time"

// nowUTC is overridable in tests.
var nowUTC = func() time.Time { return time.Now().UTC() }
