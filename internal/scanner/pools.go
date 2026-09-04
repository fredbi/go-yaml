package scanner

import "sync"

var (
	ctxPool = sync.Pool{
		New: func() interface{} {
			return createContext()
		},
	}
)
