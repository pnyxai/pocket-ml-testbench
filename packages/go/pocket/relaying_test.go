package pocket

import "testing"

func TestRelayInputCopiesHeader(t *testing.T) {
	payload := Payload{
		Data:   `{"model":"m"}`,
		Method: "POST",
		Path:   "/v1/chat/completions",
		Header: map[string][]string{
			"x-session-id": {"session-1"},
		},
	}

	input := relayInput(payload)

	if got := input.Header["x-session-id"]; len(got) != 1 || got[0] != "session-1" {
		t.Fatalf("header not copied into relay input: %#v", input.Header)
	}
	if input.Method != payload.Method || input.Path != payload.Path || string(input.Body) != payload.Data {
		t.Fatalf("relay input lost method/path/body: %#v", input)
	}
}

func TestRelayInputWithoutHeader(t *testing.T) {
	input := relayInput(Payload{Method: "POST", Path: "/v1/chat/completions"})
	if input.Header != nil {
		t.Fatalf("expected nil header, got %#v", input.Header)
	}
}
