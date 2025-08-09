package lib

import "sync"

// Global mutex to ensure sequential execution (APT uses global state)
var aptMutex sync.Mutex
