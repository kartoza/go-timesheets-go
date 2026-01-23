package util

import "fyne.io/fyne/v2"

// RunAsync executes work in a goroutine and calls onComplete with the result
// The onComplete callback is executed on the Fyne main thread for safe UI updates
func RunAsync[T any](work func() (T, error), onComplete func(T, error)) {
	go func() {
		result, err := work()
		// Use fyne.Do to ensure UI updates happen on the main thread
		fyne.Do(func() {
			onComplete(result, err)
		})
	}()
}

// RunAsyncVoid executes work in a goroutine without a return value
// The onComplete callback is executed on the Fyne main thread for safe UI updates
func RunAsyncVoid(work func() error, onComplete func(error)) {
	go func() {
		err := work()
		fyne.Do(func() {
			onComplete(err)
		})
	}()
}

// RunAsyncSimple executes work in a goroutine without error handling
func RunAsyncSimple(work func()) {
	go work()
}
