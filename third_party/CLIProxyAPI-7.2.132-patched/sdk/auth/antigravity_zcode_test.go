package auth

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestAntigravityCallbackIsLoopbackAndRejectsInvalidRequests(t *testing.T) {
	probe, errListen := net.Listen("tcp4", "127.0.0.1:0")
	if errListen != nil {
		t.Fatalf("choose callback port: %v", errListen)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()

	const state = "expected-state"
	server, actualPort, results, errStart := startAntigravityCallbackServer(port, state)
	if errStart != nil {
		t.Fatalf("start callback server: %v", errStart)
	}
	defer server.Close()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d/oauth-callback", actualPort)

	postReq, _ := http.NewRequest(http.MethodPost, baseURL, nil)
	postResp, errPost := http.DefaultClient.Do(postReq)
	if errPost != nil || postResp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("POST callback status=%v err=%v", responseStatus(postResp), errPost)
	}
	_ = postResp.Body.Close()

	invalidResp, errInvalid := http.Get(baseURL + "?state=wrong&code=code")
	if errInvalid != nil || invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid state status=%v err=%v", responseStatus(invalidResp), errInvalid)
	}
	_ = invalidResp.Body.Close()
	select {
	case <-results:
		t.Fatal("invalid state reached callback channel")
	default:
	}

	validResp, errValid := http.Get(baseURL + "?state=" + state + "&code=code")
	if errValid != nil || validResp.StatusCode != http.StatusOK {
		t.Fatalf("valid callback status=%v err=%v", responseStatus(validResp), errValid)
	}
	_ = validResp.Body.Close()
	select {
	case result := <-results:
		if result.Code != "code" || result.State != state {
			t.Fatalf("unexpected callback: %+v", result)
		}
	case <-time.After(time.Second):
		t.Fatal("valid callback was not delivered")
	}
}

func responseStatus(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	return resp.StatusCode
}
