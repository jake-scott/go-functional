package functional

// ReduceFunc is a generic function that takes two arguments and reduces them
// to a single value.
type ReduceFunc[T any, U any] func(T, T) U
