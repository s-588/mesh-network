package logger

type LogMsgType int

const (
	LogTypeDefault LogMsgType = iota
	LogTypeDATASent
	LogTypeDATAReceived
	LogTypeRREQSent
	LogTypeRREQReceived
	LogTypeRREPSent
	LogTypeRREPReceived
	LogTypeRRERSent
	LogTypeRRERReceived
)
