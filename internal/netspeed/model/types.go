package model

import "encoding/json"

type MessageType string

const (
	MsgStart    MessageType = "start"
	MsgStop     MessageType = "stop"
	MsgProgress MessageType = "progress"
	MsgResult   MessageType = "result"
	MsgPing     MessageType = "ping"
	MsgPong     MessageType = "pong"
	MsgError    MessageType = "error"
)

type TestMode string

const (
	ModeDownload      TestMode = "download"
	ModeUpload        TestMode = "upload"
	ModeBidirectional TestMode = "bidirectional"
)

type Message struct {
	Type        MessageType `json:"type"`
	Mode        TestMode    `json:"mode,omitempty"`
	Duration    int         `json:"duration,omitempty"`
	Direction   string      `json:"direction,omitempty"`
	SpeedMbps   float64     `json:"speed_mbps,omitempty"`
	TotalMB     float64     `json:"total_mb,omitempty"`
	Elapsed     float64     `json:"elapsed,omitempty"`
	AvgMbps     float64     `json:"avg_mbps,omitempty"`
	MaxMbps     float64     `json:"max_mbps,omitempty"`
	MinMbps     float64     `json:"min_mbps,omitempty"`
	TotalBytes  int64       `json:"total_bytes,omitempty"`
	DurationAct float64     `json:"duration_act,omitempty"`
	Timestamp   int64       `json:"timestamp,omitempty"`
	Text        string      `json:"text,omitempty"`
}

func ParseMessage(data []byte) (*Message, error) {
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}
