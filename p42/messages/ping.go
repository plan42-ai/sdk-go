package messages

import "encoding/json"

type PingRequest struct{}

func (p *PingRequest) Type() MessageType {
	return PingRequestMessage
}

func (p PingRequest) MarshalJSON() (data []byte, err error) {
	return json.Marshal(emptyMessage(PingRequestMessage))
}

type PingResponse struct{}

func (p *PingResponse) Type() MessageType {
	return PingResponseMessage
}

func (p *PingResponse) MarshalJSON() (data []byte, err error) {
	return json.Marshal(emptyMessage(PingResponseMessage))
}

func emptyMessage(t MessageType) any {
	var ret struct {
		Type MessageType
	}
	ret.Type = t
	return ret
}
