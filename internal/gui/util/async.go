package util

// RunAsync executes work in a goroutine and calls onComplete with the result
// Fyne handles thread safety for UI updates, so onComplete can update widgets directly
func RunAsync[T any](work func() (T, error), onComplete func(T, error)) {
	go func() {
		result, err := work()
		onComplete(result, err)
	}()
}

// RunAsyncVoid executes work in a goroutine without a return value
func RunAsyncVoid(work func() error, onComplete func(error)) {
	go func() {
		err := work()
		onComplete(err)
	}()
}

// RunAsyncSimple executes work in a goroutine without error handling
func RunAsyncSimple(work func()) {
	go work()
}
