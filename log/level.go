package log

type LogLevel int

const (
	TRACE LogLevel = iota
	DEBUG
	INFO
	WARNING
	ERROR
)
