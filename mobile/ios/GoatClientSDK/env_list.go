//go:build ios

package GoatClientSDK

// EnvList is a string-keyed string-valued bag passed across the gomobile
// boundary. gomobile cannot bind native Go maps, so callers (Swift) build up
// the bag via Put(...) and the Go side iterates via AllItems internally.
//
// Used by Run() to pre-set process environment variables before the tunnel
// goroutine starts (e.g. log level, feature flags).
type EnvList struct {
	data map[string]string
}

// NewEnvList allocates an empty EnvList. gomobile-callable.
func NewEnvList() *EnvList {
	return &EnvList{data: make(map[string]string)}
}

// Put adds (or overwrites) a key=value entry. gomobile-callable.
func (el *EnvList) Put(key, value string) {
	el.data[key] = value
}

// Get returns the value for key, or empty string if absent. gomobile-callable.
func (el *EnvList) Get(key string) string {
	return el.data[key]
}

// allItems is internal — gomobile cannot bind Go maps directly. Used by
// applyEnv() in client.go to set os.Setenv before the tunnel starts.
func (el *EnvList) allItems() map[string]string {
	if el == nil {
		return nil
	}
	return el.data
}
